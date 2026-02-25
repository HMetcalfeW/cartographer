# Roadmap

## v0.3.0 — Output Formats & Image Rendering ✅ Released

### Mermaid Output ✅
`--output-format mermaid` generates Mermaid diagram syntax. Renders natively in GitHub READMEs, Notion, Confluence, and most documentation platforms.

### JSON Output ✅
`--output-format json` emits a structured JSON representation of the dependency graph. Enables integration with other tools, custom visualizers, and CI pipelines.

### Built-in Image Rendering ✅
`--output-format png` and `--output-format svg` render images directly via GraphViz. Falls back gracefully with an install hint if GraphViz is not found.

---

## v0.4.0 — Dependency Analysis Improvements

### matchExpressions Support ✅
Extend label selector matching beyond `matchLabels` to support `matchExpressions` operators: `In`, `NotIn`, `Exists`, and `DoesNotExist`. Covers the full Kubernetes label selector spec for NetworkPolicies and PodDisruptionBudgets. (Service `.spec.selector` is a flat map and does not use `matchExpressions` per the K8s spec.)

### Cross-Namespace Resolution
Track resource namespaces and resolve references across namespace boundaries. Enables accurate graphing of Ingress routes, NetworkPolicy peers, and other cross-namespace relationships.

### RBAC Graph Edges
Extend the dependency graph to link ServiceAccounts through RoleBindings and ClusterRoleBindings to their associated Roles and ClusterRoles. Visualizes the full permission chain.

---

## v1.0.0 — Ecosystem & Extensibility

### Kustomize Support
Accept `kustomization.yaml` as input, render overlays, and analyze the resulting manifests. Covers the third major Kubernetes manifest workflow alongside raw YAML and Helm.

### CRD Plugin System
Allow users to define custom reference paths for Custom Resource Definitions via configuration. For example, map FluxCD `HelmRelease` → `spec.chart.spec.sourceRef` or cert-manager `Certificate` → `spec.issuerRef` without requiring code changes.

### Live Cluster Mode
Connect to a running Kubernetes cluster via kubeconfig and analyze deployed resources directly from the API server. Reuses the existing dependency analysis engine with a new resource source.

---

## Future

### Interactive Web Viewer
Generate a self-contained HTML file with an interactive graph (D3.js or Cytoscape.js) supporting pan, zoom, filtering, and node inspection. Eliminates the GraphViz dependency entirely for visual exploration.
