#!/usr/bin/env bash
# Integration test: validates cartographer --cluster mode against a running kind cluster.
# Prerequisites: run `make integration-cluster-up` first (or `make integration-test` for all-in-one).
# Usage: ./tests/integration/cluster_test.sh

set -euo pipefail

NAMESPACE="cartographer-test"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="$ROOT_DIR/build/integration-test/cartographer"
PASS=0
FAIL=0

# ---------- helpers ----------

red()   { printf '\033[1;31m%s\033[0m\n' "$*"; }
green() { printf '\033[1;32m%s\033[0m\n' "$*"; }
bold()  { printf '\033[1m%s\033[0m\n' "$*"; }

assert_contains() {
    local label="$1" haystack="$2" needle="$3"
    if echo "$haystack" | grep -qF -- "$needle"; then
        green "  PASS: $label"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $label — expected to find '$needle'"
        FAIL=$((FAIL + 1))
    fi
}

assert_not_contains() {
    local label="$1" haystack="$2" needle="$3"
    if echo "$haystack" | grep -qF -- "$needle"; then
        red "  FAIL: $label — did NOT expect to find '$needle'"
        FAIL=$((FAIL + 1))
    else
        green "  PASS: $label"
        PASS=$((PASS + 1))
    fi
}

# ---------- preflight ----------

bold "=== Cartographer Integration Test ==="
echo ""

if [ ! -x "$BINARY" ]; then
    red "Binary not found at $BINARY"
    red "Run 'make integration-cluster-up' first."
    exit 1
fi

# ---------- test 1: namespace-scoped JSON ----------

bold "1. Test: --cluster --namespace $NAMESPACE --output-format json"

JSON_OUT=$("$BINARY" analyze --cluster --namespace "$NAMESPACE" --output-format json 2>/dev/null)

assert_contains "JSON is valid"             "$JSON_OUT" '"nodes"'
assert_contains "Deployment/web present"    "$JSON_OUT" '"Deployment/web"'
assert_contains "Service/web-svc present"   "$JSON_OUT" '"Service/web-svc"'
assert_contains "Secret/db-creds present"   "$JSON_OUT" '"Secret/db-creds"'
assert_contains "ConfigMap/app-config"      "$JSON_OUT" '"ConfigMap/app-config"'
assert_contains "ServiceAccount/app-sa"     "$JSON_OUT" '"ServiceAccount/app-sa"'
assert_contains "RoleBinding/app-binding"   "$JSON_OUT" '"RoleBinding/app-binding"'
assert_contains "Role/app-role"             "$JSON_OUT" '"Role/app-role"'
assert_contains "HPA/web-hpa"              "$JSON_OUT" '"HorizontalPodAutoscaler/web-hpa"'

# Verify edges.
assert_contains "secretRef edge"   "$JSON_OUT" '"secretRef"'
assert_contains "configMapRef edge" "$JSON_OUT" '"configMapRef"'
assert_contains "selector edge"    "$JSON_OUT" '"selector"'
assert_contains "roleRef edge"     "$JSON_OUT" '"roleRef"'
assert_contains "subject edge"     "$JSON_OUT" '"subject"'
assert_contains "scaleTargetRef"   "$JSON_OUT" '"scaleTargetRef"'
assert_contains "serviceAccountName edge" "$JSON_OUT" '"serviceAccountName"'

# Verify group/category fields.
assert_contains "workloads group"    "$JSON_OUT" '"workloads"'
assert_contains "networking group"   "$JSON_OUT" '"networking"'
assert_contains "config group"       "$JSON_OUT" '"config"'
assert_contains "rbac group"         "$JSON_OUT" '"rbac"'
assert_contains "autoscaling group"  "$JSON_OUT" '"autoscaling"'

# Verify cluster-scoped resources do NOT leak into namespace-scoped queries.
assert_not_contains "no ClusterRoles in ns mode"        "$JSON_OUT" '"ClusterRole/'
assert_not_contains "no ClusterRoleBindings in ns mode" "$JSON_OUT" '"ClusterRoleBinding/'

echo ""

# ---------- test 2: DOT output ----------

bold "2. Test: --cluster --namespace $NAMESPACE --output-format dot"

DOT_OUT=$("$BINARY" analyze --cluster --namespace "$NAMESPACE" --output-format dot 2>/dev/null)

assert_contains "DOT header"       "$DOT_OUT" "digraph G {"
assert_contains "DOT has edges"    "$DOT_OUT" "->"
assert_contains "DOT Deployment"   "$DOT_OUT" '"Deployment/web"'
assert_contains "DOT fillcolor"    "$DOT_OUT" "fillcolor="

echo ""

# ---------- test 3: Mermaid output ----------

bold "3. Test: --cluster --namespace $NAMESPACE --output-format mermaid"

MERMAID_OUT=$("$BINARY" analyze --cluster --namespace "$NAMESPACE" --output-format mermaid 2>/dev/null)

assert_contains "Mermaid header"   "$MERMAID_OUT" "graph LR"
assert_contains "Mermaid edges"    "$MERMAID_OUT" "-->"
assert_contains "Mermaid classDef" "$MERMAID_OUT" "classDef workloads"

echo ""

# ---------- test 4: all-namespaces ----------

bold "4. Test: --cluster -A --output-format json"

ALL_NS_OUT=$("$BINARY" analyze --cluster -A --output-format json 2>/dev/null)

assert_contains "All-ns has nodes"      "$ALL_NS_OUT" '"nodes"'
assert_contains "All-ns has our deploy" "$ALL_NS_OUT" '"Deployment/web"'
assert_contains "All-ns has ClusterRoles"        "$ALL_NS_OUT" '"ClusterRole/'
assert_contains "All-ns has ClusterRoleBindings" "$ALL_NS_OUT" '"ClusterRoleBinding/'

echo ""

# ---------- test 5: config-driven filtering ----------

bold "5. Test: config-driven exclusion (exclude Service kind)"

FILTER_CONFIG=$(mktemp /tmp/cartographer-test-XXXXXX.yaml)
cat > "$FILTER_CONFIG" <<'EOF'
exclude:
  kinds:
    - Service
EOF

FILTERED_OUT=$("$BINARY" --config "$FILTER_CONFIG" analyze --cluster --namespace "$NAMESPACE" --output-format json 2>/dev/null)
rm -f "$FILTER_CONFIG"

assert_not_contains "Service excluded by kind" "$FILTERED_OUT" '"Service/web-svc"'
assert_contains "Deployment still present"     "$FILTERED_OUT" '"Deployment/web"'

echo ""

# ---------- test 6: name exclusion ----------

bold "6. Test: config-driven exclusion (exclude web-svc by name)"

NAME_CONFIG=$(mktemp /tmp/cartographer-test-XXXXXX.yaml)
cat > "$NAME_CONFIG" <<'EOF'
exclude:
  names:
    - web-svc
EOF

NAME_OUT=$("$BINARY" --config "$NAME_CONFIG" analyze --cluster --namespace "$NAMESPACE" --output-format json 2>/dev/null)
rm -f "$NAME_CONFIG"

assert_not_contains "web-svc excluded by name" "$NAME_OUT" '"Service/web-svc"'
assert_contains "Deployment still present"      "$NAME_OUT" '"Deployment/web"'

echo ""

# ---------- test 7: mutual exclusivity ----------

bold "7. Test: --cluster + --input mutual exclusivity"

MUTEX_OUT=$("$BINARY" analyze --cluster --input /dev/null 2>&1 || true)
if echo "$MUTEX_OUT" | grep -qF "mutually exclusive"; then
    green "  PASS: mutually exclusive error"
    PASS=$((PASS + 1))
else
    red "  FAIL: expected mutually exclusive error"
    FAIL=$((FAIL + 1))
fi

echo ""

# ---------- summary ----------

bold "=== Results ==="
green "Passed: $PASS"
if [ "$FAIL" -gt 0 ]; then
    red "Failed: $FAIL"
    exit 1
else
    green "All integration tests passed!"
fi
