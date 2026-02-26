package dependency_test

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/HMetcalfeW/cartographer/pkg/parser"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func init() {
	// If you want minimal logging noise during tests:
	log.SetLevel(log.ErrorLevel)
}

// TestBuildDependencies verifies the main BuildDependencies function end-to-end.
func TestBuildDependencies(t *testing.T) {
	// Create a Deployment referencing a Secret, ServiceAccount, etc.
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "my-deploy",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"kind": "HelmRelease",
						"name": "my-release",
					},
				},
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"serviceAccountName": "my-sa",
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "secret-vol",
								"secret": map[string]interface{}{
									"secretName": "my-secret",
								},
							},
						},
						"containers": []interface{}{
							map[string]interface{}{
								"name": "web",
								"env": []interface{}{
									map[string]interface{}{
										"name": "CONFIG",
										"valueFrom": map[string]interface{}{
											"configMapKeyRef": map[string]interface{}{
												"name": "my-cm",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// A Service that selects Pods with label app=webapp
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "my-service",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"app": "webapp",
				},
			},
		},
	}

	// A Pod that is selected by the Service
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "my-pod",
				"labels": map[string]interface{}{
					"app": "webapp",
				},
			},
		},
	}

	// An Ingress referencing this service
	ing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata": map[string]interface{}{
				"name": "my-ing",
			},
			"spec": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"http": map[string]interface{}{
							"paths": []interface{}{
								map[string]interface{}{
									"path": "/",
									"backend": map[string]interface{}{
										"service": map[string]interface{}{
											"name": "my-service",
											"port": map[string]interface{}{
												"number": float64(80),
											},
										},
									},
								},
							},
						},
					},
				},
				"tls": []interface{}{
					map[string]interface{}{
						"secretName": "tls-secret",
					},
				},
			},
		},
	}

	// A HorizontalPodAutoscaler referencing the Deployment
	hpa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]interface{}{
				"name": "my-hpa",
			},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
				},
			},
		},
	}

	// Additional resources
	helmRelease := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.example.com/v1",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name": "my-release",
			},
		},
	}
	secret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "my-secret",
			},
		},
	}
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "my-cm",
			},
		},
	}
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name": "my-sa",
			},
		},
	}
	tlsSecret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "tls-secret",
			},
		},
	}

	objs := []*unstructured.Unstructured{
		deployment, service, pod, ing, hpa,
		helmRelease, secret, cm, sa, tlsSecret,
	}

	deps := dependency.BuildDependencies(objs)

	// Confirm the HelmRelease -> Deployment
	hrEdges := deps["HelmRelease/my-release"]
	require.Len(t, hrEdges, 1)
	assert.Equal(t, "Deployment/my-deploy", hrEdges[0].ChildID)
	assert.Equal(t, "ownerRef", hrEdges[0].Reason)

	// Confirm the Deployment -> Secret, ConfigMap, ServiceAccount
	depEdges := deps["Deployment/my-deploy"]
	require.Len(t, depEdges, 3, "expected 3 references from Deployment/my-deploy")

	var secretRef, cmRef, saRef bool
	for _, e := range depEdges {
		if e.ChildID == "Secret/my-secret" && e.Reason == "secretRef" {
			secretRef = true
		}
		if e.ChildID == "ConfigMap/my-cm" && e.Reason == "configMapRef" {
			cmRef = true
		}
		if e.ChildID == "ServiceAccount/my-sa" && e.Reason == "serviceAccountName" {
			saRef = true
		}
	}
	assert.True(t, secretRef, "Expected secretRef to my-secret")
	assert.True(t, cmRef, "Expected configMapRef to my-cm")
	assert.True(t, saRef, "Expected serviceAccountName to my-sa")

	// Confirm the Service -> Pod (label selector)
	svcEdges := deps["Service/my-service"]
	require.Len(t, svcEdges, 1)
	assert.Equal(t, "Pod/my-pod", svcEdges[0].ChildID)
	assert.Equal(t, "selector", svcEdges[0].Reason)

	// Confirm the Ingress references
	ingEdges := deps["Ingress/my-ing"]
	require.Len(t, ingEdges, 2, "expected 2 edges from Ingress: service, secret")
	var svcFound, tlsFound bool
	for _, e := range ingEdges {
		if e.ChildID == "Service/my-service" && e.Reason == "ingressBackend" {
			svcFound = true
		}
		if e.ChildID == "Secret/tls-secret" && e.Reason == "tlsSecret" {
			tlsFound = true
		}
	}
	assert.True(t, svcFound, "Expected ingressBackend to my-service")
	assert.True(t, tlsFound, "Expected tlsSecret to Secret/tls-secret")

	// Confirm the HPA
	hpaEdges := deps["HorizontalPodAutoscaler/my-hpa"]
	require.Len(t, hpaEdges, 1)
	assert.Equal(t, "Deployment/my-deploy", hpaEdges[0].ChildID)
	assert.Equal(t, "scaleTargetRef", hpaEdges[0].Reason)
}

// TestBuildDependencies_IntegrationManifest tests the full pipeline: YAML parsing → dependency analysis.
// This exercises the same code path as `cartographer analyze --input` without needing external files.
func TestBuildDependencies_IntegrationManifest(t *testing.T) {
	manifest := `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: web-sa
---
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
type: Opaque
data:
  password: cGFzc3dvcmQ=
---
apiVersion: v1
kind: Secret
metadata:
  name: tls-cert
type: kubernetes.io/tls
data:
  tls.crt: ""
  tls.key: ""
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  APP_ENV: production
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      serviceAccountName: web-sa
      containers:
        - name: web
          image: nginx:latest
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: password
          envFrom:
            - configMapRef:
                name: app-config
      volumes:
        - name: tls
          secret:
            secretName: tls-cert
---
apiVersion: v1
kind: Service
metadata:
  name: web-svc
spec:
  selector:
    app: web
  ports:
    - port: 80
      targetPort: 8080
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web-ingress
spec:
  tls:
    - secretName: tls-cert
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web-svc
                port:
                  number: 80
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  minReplicas: 2
  maxReplicas: 10
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: web-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: web
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: web-netpol
spec:
  podSelector:
    matchLabels:
      app: web
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: web
`

	objs, err := parser.ParseYAML([]byte(manifest))
	require.NoError(t, err)
	require.Len(t, objs, 10, "expected 10 resources in manifest")

	deps := dependency.BuildDependencies(objs)

	// Deployment → Secret (volume), Secret (env secretKeyRef), ConfigMap (envFrom), ServiceAccount
	deployEdges := deps["Deployment/web"]
	edgeSet := make(map[string]string)
	for _, e := range deployEdges {
		edgeSet[e.ChildID] = e.Reason
	}
	assert.Equal(t, "secretRef", edgeSet["Secret/tls-cert"], "Deployment should ref tls-cert volume")
	assert.Equal(t, "secretRef", edgeSet["Secret/db-credentials"], "Deployment should ref db-credentials env")
	assert.Equal(t, "configMapRef", edgeSet["ConfigMap/app-config"], "Deployment should ref app-config envFrom")
	assert.Equal(t, "serviceAccountName", edgeSet["ServiceAccount/web-sa"], "Deployment should ref web-sa")

	// Service → Deployment (selector)
	svcEdges := deps["Service/web-svc"]
	require.Len(t, svcEdges, 1)
	assert.Equal(t, "Deployment/web", svcEdges[0].ChildID)
	assert.Equal(t, "selector", svcEdges[0].Reason)

	// Ingress → Service + TLS Secret
	ingEdges := deps["Ingress/web-ingress"]
	require.Len(t, ingEdges, 2)
	ingEdgeSet := make(map[string]string)
	for _, e := range ingEdges {
		ingEdgeSet[e.ChildID] = e.Reason
	}
	assert.Equal(t, "ingressBackend", ingEdgeSet["Service/web-svc"])
	assert.Equal(t, "tlsSecret", ingEdgeSet["Secret/tls-cert"])

	// HPA → Deployment
	hpaEdges := deps["HorizontalPodAutoscaler/web-hpa"]
	require.Len(t, hpaEdges, 1)
	assert.Equal(t, "Deployment/web", hpaEdges[0].ChildID)
	assert.Equal(t, "scaleTargetRef", hpaEdges[0].Reason)

	// PDB → Deployment (pdbSelector)
	pdbEdges := deps["PodDisruptionBudget/web-pdb"]
	require.Len(t, pdbEdges, 1)
	assert.Equal(t, "Deployment/web", pdbEdges[0].ChildID)
	assert.Equal(t, "pdbSelector", pdbEdges[0].Reason)

	// NetworkPolicy → Deployment (podSelector)
	npEdges := deps["NetworkPolicy/web-netpol"]
	require.Len(t, npEdges, 1)
	assert.Equal(t, "Deployment/web", npEdges[0].ChildID)
	assert.Equal(t, "podSelector", npEdges[0].Reason)

	// Verify DOT output is valid
	dot := dependency.GenerateDOT(deps)
	assert.Contains(t, dot, "digraph G {")
	assert.Contains(t, dot, `"Service/web-svc" -> "Deployment/web"`)
	assert.Contains(t, dot, `"Ingress/web-ingress" -> "Service/web-svc"`)
}

// TestBuildDependencies_MatchExpressions tests matchExpressions across multiple
// resource types with various operators.
func TestBuildDependencies_MatchExpressions(t *testing.T) {
	manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
    tier: frontend
    env: prod
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  labels:
    app: api
    tier: backend
    env: prod
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  labels:
    app: worker
    tier: backend
    env: staging
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: prod-only
spec:
  podSelector:
    matchExpressions:
      - key: env
        operator: In
        values: [prod]
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: backend-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      tier: backend
    matchExpressions:
      - key: env
        operator: NotIn
        values: [staging]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: all-with-app
spec:
  podSelector:
    matchExpressions:
      - key: app
        operator: Exists
`

	objs, err := parser.ParseYAML([]byte(manifest))
	require.NoError(t, err)
	require.Len(t, objs, 6)

	deps := dependency.BuildDependencies(objs)

	// NetworkPolicy/prod-only → web + api (env In [prod])
	npProdEdges := deps["NetworkPolicy/prod-only"]
	require.Len(t, npProdEdges, 2, "prod-only should match web and api")
	npProdTargets := map[string]bool{}
	for _, e := range npProdEdges {
		npProdTargets[e.ChildID] = true
		assert.Equal(t, "podSelector", e.Reason)
	}
	assert.True(t, npProdTargets["Deployment/web"])
	assert.True(t, npProdTargets["Deployment/api"])
	assert.False(t, npProdTargets["Deployment/worker"])

	// PDB/backend-pdb → api only (tier=backend AND env NotIn [staging])
	pdbEdges := deps["PodDisruptionBudget/backend-pdb"]
	require.Len(t, pdbEdges, 1, "backend-pdb: tier=backend AND env NotIn [staging] → api only")
	assert.Equal(t, "Deployment/api", pdbEdges[0].ChildID)
	assert.Equal(t, "pdbSelector", pdbEdges[0].Reason)

	// NetworkPolicy/all-with-app → web + api + worker (app Exists)
	npAllEdges := deps["NetworkPolicy/all-with-app"]
	require.Len(t, npAllEdges, 3, "all-with-app should match all three deployments")
}

// TestBuildDependencies_HelmStyleMatchExpressions exercises matchExpressions
// with app.kubernetes.io/* labels typical of Bitnami/Helm charts.
func TestBuildDependencies_HelmStyleMatchExpressions(t *testing.T) {
	manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-master
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/component: master
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-replicas
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/component: replica
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  labels:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: primary
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: redis-master-only
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: redis
    matchExpressions:
      - key: app.kubernetes.io/component
        operator: In
        values: [master]
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: redis-all
spec:
  minAvailable: 1
  selector:
    matchExpressions:
      - key: app.kubernetes.io/name
        operator: In
        values: [redis]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: not-postgres
spec:
  podSelector:
    matchExpressions:
      - key: app.kubernetes.io/name
        operator: NotIn
        values: [postgres]
`

	objs, err := parser.ParseYAML([]byte(manifest))
	require.NoError(t, err)
	require.Len(t, objs, 6)

	deps := dependency.BuildDependencies(objs)

	// redis-master-only → only redis-master (name=redis AND component In [master])
	npMasterEdges := deps["NetworkPolicy/redis-master-only"]
	require.Len(t, npMasterEdges, 1)
	assert.Equal(t, "Deployment/redis-master", npMasterEdges[0].ChildID)

	// redis-all PDB → both redis deployments
	pdbEdges := deps["PodDisruptionBudget/redis-all"]
	require.Len(t, pdbEdges, 2, "redis-all should match redis-master and redis-replicas")
	pdbTargets := map[string]bool{}
	for _, e := range pdbEdges {
		pdbTargets[e.ChildID] = true
	}
	assert.True(t, pdbTargets["Deployment/redis-master"])
	assert.True(t, pdbTargets["Deployment/redis-replicas"])
	assert.False(t, pdbTargets["Deployment/postgres"])

	// not-postgres → redis-master + redis-replicas (NotIn [postgres])
	npNotPgEdges := deps["NetworkPolicy/not-postgres"]
	require.Len(t, npNotPgEdges, 2)
	npNotPgTargets := map[string]bool{}
	for _, e := range npNotPgEdges {
		npNotPgTargets[e.ChildID] = true
	}
	assert.True(t, npNotPgTargets["Deployment/redis-master"])
	assert.True(t, npNotPgTargets["Deployment/redis-replicas"])
}

// TestBuildDependencies_Clustering verifies output formats include subgraph clustering.
func TestBuildDependencies_Clustering(t *testing.T) {
	manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
spec:
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      serviceAccountName: app-sa
      containers:
        - name: web
          image: nginx
          env:
            - name: DB
              valueFrom:
                secretKeyRef:
                  name: db-creds
                  key: pass
---
apiVersion: v1
kind: Secret
metadata:
  name: db-creds
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-sa
---
apiVersion: v1
kind: Service
metadata:
  name: web-svc
spec:
  selector:
    app: web
  ports:
    - port: 80
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: app-role
subjects:
  - kind: ServiceAccount
    name: app-sa
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
`

	objs, err := parser.ParseYAML([]byte(manifest))
	require.NoError(t, err)

	deps := dependency.BuildDependencies(objs)

	// DOT: verify color-coded nodes (no subgraph clusters)
	dot := dependency.GenerateDOT(deps)
	assert.Contains(t, dot, `"Deployment/web" [fillcolor=`)
	assert.Contains(t, dot, `"Service/web-svc" [fillcolor=`)
	assert.Contains(t, dot, `"Secret/db-creds" [fillcolor=`)
	assert.NotContains(t, dot, "subgraph cluster_workloads")

	// Mermaid: verify color-coded nodes (no subgraph clusters)
	mermaid := dependency.GenerateMermaid(deps)
	assert.NotContains(t, mermaid, "subgraph")
	assert.Contains(t, mermaid, "classDef workloads fill:#DAEEF3")
	assert.Contains(t, mermaid, "classDef networking fill:#E2EFDA")
	assert.Contains(t, mermaid, "classDef config fill:#FFF2CC")
	assert.Contains(t, mermaid, "classDef rbac fill:#E2D9F3")
	assert.Contains(t, mermaid, "classDef autoscaling fill:#FCE4D6")

	// JSON: verify group field
	jsonOut := dependency.GenerateJSON(deps)
	assert.Contains(t, jsonOut, `"group": "workloads"`)
	assert.Contains(t, jsonOut, `"group": "networking"`)
	assert.Contains(t, jsonOut, `"group": "config"`)
	assert.Contains(t, jsonOut, `"group": "rbac"`)
	assert.Contains(t, jsonOut, `"group": "autoscaling"`)
}

// TestBuildDependencies_RBAC tests the full ServiceAccount → RoleBinding → Role chain.
func TestBuildDependencies_RBAC(t *testing.T) {
	manifest := `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-sa
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: app-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: app-role
subjects:
  - kind: ServiceAccount
    name: app-sa
    namespace: default
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
spec:
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      serviceAccountName: app-sa
      containers:
        - name: web
          image: nginx
`

	objs, err := parser.ParseYAML([]byte(manifest))
	require.NoError(t, err)
	require.Len(t, objs, 4)

	deps := dependency.BuildDependencies(objs)

	// RoleBinding → Role (roleRef)
	rbEdges := deps["RoleBinding/app-binding"]
	edgeSet := map[string]string{}
	for _, e := range rbEdges {
		edgeSet[e.ChildID] = e.Reason
	}
	assert.Equal(t, "roleRef", edgeSet["Role/app-role"])
	assert.Equal(t, "subject", edgeSet["ServiceAccount/app-sa"])

	// Deployment → ServiceAccount
	deployEdges := deps["Deployment/web"]
	deployEdgeSet := map[string]string{}
	for _, e := range deployEdges {
		deployEdgeSet[e.ChildID] = e.Reason
	}
	assert.Equal(t, "serviceAccountName", deployEdgeSet["ServiceAccount/app-sa"])

	// Verify DOT output includes RBAC edges
	dot := dependency.GenerateDOT(deps)
	assert.Contains(t, dot, `"RoleBinding/app-binding" -> "Role/app-role"`)
	assert.Contains(t, dot, `"RoleBinding/app-binding" -> "ServiceAccount/app-sa"`)
}

// TestBuildDependencies_RBAC_HelmStyle tests RBAC with Helm-style labels and
// ClusterRoleBinding with multiple subjects.
func TestBuildDependencies_RBAC_HelmStyle(t *testing.T) {
	manifest := `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
  labels:
    app.kubernetes.io/name: prometheus
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: grafana
  labels:
    app.kubernetes.io/name: grafana
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: monitoring-reader
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes", "services"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: monitoring-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: monitoring-reader
subjects:
  - kind: ServiceAccount
    name: prometheus
    namespace: monitoring
  - kind: ServiceAccount
    name: grafana
    namespace: monitoring
  - kind: Group
    name: system:monitoring
`

	objs, err := parser.ParseYAML([]byte(manifest))
	require.NoError(t, err)
	require.Len(t, objs, 4)

	deps := dependency.BuildDependencies(objs)

	// ClusterRoleBinding → ClusterRole + 2 ServiceAccounts (Group skipped)
	crbEdges := deps["ClusterRoleBinding/monitoring-binding"]
	require.Len(t, crbEdges, 3)

	edgeSet := map[string]string{}
	for _, e := range crbEdges {
		edgeSet[e.ChildID] = e.Reason
	}
	assert.Equal(t, "roleRef", edgeSet["ClusterRole/monitoring-reader"])
	assert.Equal(t, "subject", edgeSet["ServiceAccount/prometheus"])
	assert.Equal(t, "subject", edgeSet["ServiceAccount/grafana"])

	// Verify JSON output includes RBAC edges
	jsonOut := dependency.GenerateJSON(deps)
	assert.Contains(t, jsonOut, "ClusterRoleBinding/monitoring-binding")
	assert.Contains(t, jsonOut, "ClusterRole/monitoring-reader")
	assert.Contains(t, jsonOut, "roleRef")
	assert.Contains(t, jsonOut, "subject")
}
