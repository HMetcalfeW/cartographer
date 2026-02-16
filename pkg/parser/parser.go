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

// ParseYAML parses raw YAML bytes (potentially multi-document) and returns
// a slice of unstructured Kubernetes objects. Empty documents are skipped.
func ParseYAML(data []byte) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(io.NopCloser(bytes.NewReader(data)), BUFFER_BYTES)

	var objs []*unstructured.Unstructured
	for {
		var obj map[string]interface{}
		if err := decoder.Decode(&obj); err != nil {
			break
		}
		if len(obj) == 0 {
			continue
		}
		u := &unstructured.Unstructured{Object: obj}
		objs = append(objs, u)
	}

	return objs, nil
}

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

	objs, err := ParseYAML(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML from %s: %w", path, err)
	}

	logger.Debug(objs)

	return objs, nil
}
