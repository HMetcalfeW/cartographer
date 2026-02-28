package filter_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/filter"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeObj(kind, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "default",
			},
		},
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name         string
		objs         []*unstructured.Unstructured
		excludeKinds []string
		excludeNames []string
		wantCount    int
		wantKinds    []string
	}{
		{
			name:         "empty exclusions returns all",
			objs:         []*unstructured.Unstructured{makeObj("Deployment", "web"), makeObj("ConfigMap", "cfg")},
			excludeKinds: nil,
			excludeNames: nil,
			wantCount:    2,
		},
		{
			name:         "exclude by kind",
			objs:         []*unstructured.Unstructured{makeObj("Deployment", "web"), makeObj("ConfigMap", "cfg"), makeObj("ConfigMap", "other")},
			excludeKinds: []string{"ConfigMap"},
			wantCount:    1,
			wantKinds:    []string{"Deployment"},
		},
		{
			name:         "exclude by name",
			objs:         []*unstructured.Unstructured{makeObj("ConfigMap", "kube-root-ca.crt"), makeObj("ConfigMap", "app-config")},
			excludeNames: []string{"kube-root-ca.crt"},
			wantCount:    1,
			wantKinds:    []string{"ConfigMap"},
		},
		{
			name:         "exclude by kind and name",
			objs:         []*unstructured.Unstructured{makeObj("Secret", "db-creds"), makeObj("ConfigMap", "cfg"), makeObj("Deployment", "web")},
			excludeKinds: []string{"Secret"},
			excludeNames: []string{"cfg"},
			wantCount:    1,
			wantKinds:    []string{"Deployment"},
		},
		{
			name:         "case-insensitive kind matching",
			objs:         []*unstructured.Unstructured{makeObj("ConfigMap", "cfg"), makeObj("Deployment", "web")},
			excludeKinds: []string{"configmap"},
			wantCount:    1,
			wantKinds:    []string{"Deployment"},
		},
		{
			name:         "no matches leaves all",
			objs:         []*unstructured.Unstructured{makeObj("Deployment", "web"), makeObj("Service", "svc")},
			excludeKinds: []string{"ConfigMap"},
			excludeNames: []string{"nonexistent"},
			wantCount:    2,
		},
		{
			name:         "empty input returns empty",
			objs:         []*unstructured.Unstructured{},
			excludeKinds: []string{"ConfigMap"},
			wantCount:    0,
		},
		{
			name:         "all excluded returns empty",
			objs:         []*unstructured.Unstructured{makeObj("ConfigMap", "a"), makeObj("ConfigMap", "b")},
			excludeKinds: []string{"ConfigMap"},
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Apply(tt.objs, tt.excludeKinds, tt.excludeNames)
			assert.Len(t, result, tt.wantCount)
			if tt.wantKinds != nil {
				for i, obj := range result {
					assert.Equal(t, tt.wantKinds[i], obj.GetKind())
				}
			}
		})
	}
}
