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
	logger := log.WithFields(log.Fields{
		"func":     "ParseYAMLFile",
		"filepath": path,
	})
	logger.WithField("filepath", path).Info("Parsing YAML input")

	data, err := os.ReadFile(path)
	if err != nil {
		logger.WithError(err).WithField("filepath", path).Error("failed to read YAML file")
		return nil, err
	}
	logger.WithField("filepath", path).Debug("Successfully read YAML file")

	// Create a YAML decoder that supports multi-document YAML files.
	logger.Debug("Creating YAML decoder")
	decoder := yaml.NewYAMLOrJSONDecoder(io.NopCloser(bytes.NewReader(data)), BUFFER_BYTES)
	var objs []*unstructured.Unstructured

	for {
		var obj map[string]interface{}
		logger.Debug("Decoding YAML document")
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				logger.Debug("End of YAML stream")
				break // End of file
			}
			logger.WithError(err).Error("failed to decode YAML document")
			return nil, fmt.Errorf("error: Failed to decode YAML from '%s'. Please check for malformed YAML syntax: %w", path, err)
		}
		// Skip empty documents.
		if len(obj) == 0 {
			logger.Debug("Skipping empty YAML document")
			continue
		}
		u := &unstructured.Unstructured{Object: obj}
		objs = append(objs, u)
	}

	logger.WithField("parsedObjects", len(objs)).Info("Successfully parsed YAML input")

	return objs, nil
}
