package parser

import (
	"bytes"
	"io"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ParseYAMLFile reads a YAML file and returns a slice of unstructured objects.
func ParseYAMLFile(path string) ([]*unstructured.Unstructured, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Create a YAML decoder that supports multi-document YAML files.
	decoder := yaml.NewYAMLOrJSONDecoder(io.NopCloser(bytes.NewReader(data)), 4096)
	var objs []*unstructured.Unstructured

	for {
		var obj map[string]interface{}
		if err := decoder.Decode(&obj); err != nil {
			break
		}
		// Skip empty documents.
		if len(obj) == 0 {
			continue
		}
		u := &unstructured.Unstructured{Object: obj}
		objs = append(objs, u)
	}

	return objs, nil
}
