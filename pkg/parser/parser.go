package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	BUFFER_BYTES = 4096
)

// ParseYAMLFile reads a YAML file and returns a slice of unstructured objects.
func ParseYAMLFile(path string) ([]*unstructured.Unstructured, error) {
	if path == "" {
		return nil, fmt.Errorf("file path must not be empty")
	}

	logger := log.WithFields(log.Fields{
		"func":     "ParseYAMLFile",
		"filepath": path,
	})
	logger.Info("Parsing yaml input")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Create a YAML decoder that supports multi-document YAML files.
	decoder := yaml.NewYAMLOrJSONDecoder(io.NopCloser(bytes.NewReader(data)), BUFFER_BYTES)
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

	logger.Debug(objs)

	return objs, nil
}
