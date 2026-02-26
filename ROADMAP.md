# Roadmap

## v0.3.0 — Output Formats & Image Rendering ✅ Released

### Mermaid Output ✅
`--output-format mermaid` generates Mermaid diagram syntax. Renders natively in GitHub READMEs, Notion, Confluence, and most documentation platforms.

### JSON Output ✅
`--output-format json` emits a structured JSON representation of the dependency graph. Enables integration with other tools, custom visualizers, and CI pipelines.

### Built-in Image Rendering ✅
`--output-format png` and `--output-format svg` render images directly via GraphViz. Falls back gracefully with an install hint if GraphViz is not found.

---

## v0.4.0 — matchExpressions ✅ Released

### matchExpressions Support ✅
Extend label selector matching beyond `matchLabels` to support `matchExpressions` operators: `In`, `NotIn`, `Exists`, and `DoesNotExist`. Covers the full Kubernetes label selector spec for NetworkPolicies and PodDisruptionBudgets. (Service `.spec.selector` is a flat map and does not use `matchExpressions` per the K8s spec.)

---

## v0.5.0 — Refactor, Hardening & RBAC ✅ Released

### Phase 1: Refactor & Simplify ✅

- [x] **Split `types.go` into `types.go` + `labelindex.go`**
- [x] **Consolidate `dependency.go` object loops** — single loop with switch on `obj.GetKind()`
- [x] **Extract `writeTextOutput` helper in `analyze.go`**
- [x] **Extract `pullOCIChart()` from `render.go`**
- [x] **Extract dependency update logic in `render.go`** — `updateDependencies()` function
- [x] **Inline `pullClientKeyring()`** — replaced with string literal
- [x] **Remove `PrintDependencies` from `dot.go`**
- [x] **Rename `DEFAULT_NAMESPACE` to `DefaultNamespace`**
- [x] **Update `RootCmd.Short`** — now says "Visualize dependencies between Kubernetes resources"
- [x] **Collapse pod spec edge-building** — `appendEdges()` helper

### Phase 2: Hardening ✅

- [x] **Remove dead Viper flag bindings in `analyze.go`**
- [x] **Code coverage audit** — 85.3% overall (up from 82.7% baseline)
- [x] **Add tests for `cmd/version`** — 100% coverage
- [x] **Improve `pkg/helm/render.go` coverage** — bad values, no namespace, NOTES filtering, invalid chart
- [x] **Improve handler coverage** — missing spec, missing selector, incomplete HPA, empty ingress
- [x] **AGENTS.md** — coding standards and testing expectations finalized

### Phase 3: RBAC Graph Edges ✅

- [x] **Add RBAC handler** — `handleRoleBinding()` links RoleBinding/ClusterRoleBinding → Role/ClusterRole (roleRef) + ServiceAccount (subjects)
- [x] **Register RBAC kinds in `dependency.go`** — `RoleBinding` and `ClusterRoleBinding` in dispatch switch
- [x] **Unit tests** — RoleBinding → Role + SA, ClusterRoleBinding → ClusterRole + multiple SAs, missing roleRef, missing subjects, empty binding
- [x] **Integration tests** — full YAML with SA → RoleBinding → Role chain, Helm-style with ClusterRoleBinding
- [x] **CLI integration tests** — RBAC manifest across all 5 output formats (JSON, DOT, Mermaid, PNG, SVG)
- [x] **README.md updated** — RoleBinding/ClusterRoleBinding added to Supported Resource Types
- [x] **ROADMAP.md updated** — v0.5.0 marked complete

---

## v0.6.0 — Cross-Namespace Resolution

### Cross-Namespace Resolution
Track resource namespaces and resolve references across namespace boundaries. Enables accurate graphing of Ingress routes, NetworkPolicy peers, and other cross-namespace relationships. This is a correctness improvement — the current single-namespace assumption silently produces incomplete graphs for multi-namespace manifests.

---

## v0.7.0 — Kustomize Support

### Kustomize Support
Accept `kustomization.yaml` as input, render overlays, and analyze the resulting manifests. Completes the "three input sources" story: raw YAML, Helm, and Kustomize — covering every mainstream Kubernetes manifest workflow.

---

## v1.0.0 — CRD Plugin System & Stabilization

### CRD Plugin System
Allow users to define custom reference paths for Custom Resource Definitions via configuration. For example, map FluxCD `HelmRelease` → `spec.chart.spec.sourceRef` or cert-manager `Certificate` → `spec.issuerRef` without requiring code changes. This is the 1.0 gate — after this, users can handle any resource type without waiting for upstream changes.

### Stabilization
Final pass on error messages, `--verbose`/`--quiet` flags, documentation polish, and edge case hardening. Ensure the CLI is production-ready for CI/CD pipelines and team workflows.

---

## Post-1.0

### Live Cluster Mode
Connect to a running Kubernetes cluster via kubeconfig and analyze deployed resources directly from the API server. Reuses the existing dependency analysis engine with a new resource source.

### Interactive Web Viewer
Generate a self-contained HTML file with an interactive graph (D3.js or Cytoscape.js) supporting pan, zoom, filtering, and node inspection. Eliminates the GraphViz dependency entirely for visual exploration.
