package dependency

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategoryForNode(t *testing.T) {
	tests := []struct {
		name     string
		nodeID   string
		expected string
	}{
		{"Deployment is workloads", "Deployment/web", "workloads"},
		{"Pod is workloads", "Pod/runner", "workloads"},
		{"CronJob is workloads", "CronJob/backup", "workloads"},
		{"Service is networking", "Service/frontend", "networking"},
		{"Ingress is networking", "Ingress/main", "networking"},
		{"NetworkPolicy is networking", "NetworkPolicy/deny-all", "networking"},
		{"ConfigMap is config", "ConfigMap/app-config", "config"},
		{"Secret is config", "Secret/db-creds", "config"},
		{"PVC is config", "PersistentVolumeClaim/data", "config"},
		{"Role is rbac", "Role/reader", "rbac"},
		{"ClusterRole is rbac", "ClusterRole/admin", "rbac"},
		{"RoleBinding is rbac", "RoleBinding/bind-reader", "rbac"},
		{"ClusterRoleBinding is rbac", "ClusterRoleBinding/bind-admin", "rbac"},
		{"ServiceAccount is rbac", "ServiceAccount/app-sa", "rbac"},
		{"HPA is autoscaling", "HorizontalPodAutoscaler/web-hpa", "autoscaling"},
		{"PDB is autoscaling", "PodDisruptionBudget/web-pdb", "autoscaling"},
		{"Unknown kind is other", "CustomResource/foo", "other"},
		{"No slash falls back to other", "orphan", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CategoryForNode(tt.nodeID))
		})
	}
}

func TestGroupNodesByCategory(t *testing.T) {
	deps := map[string][]Edge{
		"Deployment/web": {
			{ChildID: "Secret/db-creds", Reason: "secretRef"},
			{ChildID: "ConfigMap/app-config", Reason: "configMapRef"},
		},
		"Service/frontend": {
			{ChildID: "Deployment/web", Reason: "selector"},
		},
		"RoleBinding/bind-reader": {
			{ChildID: "Role/reader", Reason: "roleRef"},
			{ChildID: "ServiceAccount/app-sa", Reason: "subject"},
		},
		"HorizontalPodAutoscaler/web-hpa": {
			{ChildID: "Deployment/web", Reason: "scaleTargetRef"},
		},
	}

	groups := GroupNodesByCategory(deps)

	assert.Equal(t, []string{"Deployment/web"}, groups["workloads"])
	assert.Equal(t, []string{"Service/frontend"}, groups["networking"])
	assert.Equal(t, []string{"ConfigMap/app-config", "Secret/db-creds"}, groups["config"])
	assert.Equal(t, []string{"Role/reader", "RoleBinding/bind-reader", "ServiceAccount/app-sa"}, groups["rbac"])
	assert.Equal(t, []string{"HorizontalPodAutoscaler/web-hpa"}, groups["autoscaling"])
	assert.Empty(t, groups["other"])
}
