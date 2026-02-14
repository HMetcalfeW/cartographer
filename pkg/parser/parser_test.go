package parser_test

import (
	"os"
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYAMLFile(t *testing.T) {
	yamlContent := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
`

	// Create a temporary YAML file using os.CreateTemp.
	tmpfile, err := os.CreateTemp("", "test-*.yaml")
	require.NoError(t, err, "Failed to create temp file")

	// Defer removal of the temp file and log any errors.
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}()

	// Write the YAML content to the temporary file.
	_, err = tmpfile.Write([]byte(yamlContent))
	require.NoError(t, err, "Failed to write to temp file")

	err = tmpfile.Close()
	require.NoError(t, err, "Failed to close temp file")

	// Invoke the parser.
	objs, err := parser.ParseYAMLFile(tmpfile.Name())
	require.NoError(t, err, "ParseYAMLFile returned an error")

	// We expect exactly 2 objects.
	require.Equal(t, 2, len(objs), "Expected 2 objects")

	// Verify that one object is a Pod and the other is a Deployment.
	var foundPod, foundDeployment bool
	for _, obj := range objs {
		switch obj.GetKind() {
		case "Pod":
			foundPod = true
		case "Deployment":
			foundDeployment = true
		}
	}

	assert.True(t, foundPod, "Expected to find a Pod object")
	assert.True(t, foundDeployment, "Expected to find a Deployment object")
}

func TestParseYAMLFile_EmptyPath(t *testing.T) {
	_, err := parser.ParseYAMLFile("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file path must not be empty")
}

func TestParseYAMLFile_NonexistentFile(t *testing.T) {
	_, err := parser.ParseYAMLFile("/tmp/definitely-does-not-exist-12345.yaml")
	require.Error(t, err)
}

func TestParseYAMLFile_EmptyDocuments(t *testing.T) {
	// YAML with empty documents separated by ---
	yamlContent := `---
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: only-one
---
---
`
	tmpfile, err := os.CreateTemp("", "test-empty-docs-*.yaml")
	require.NoError(t, err)
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}()

	_, err = tmpfile.Write([]byte(yamlContent))
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	objs, err := parser.ParseYAMLFile(tmpfile.Name())
	require.NoError(t, err)
	require.Len(t, objs, 1, "should skip empty documents and parse only the ConfigMap")
	assert.Equal(t, "ConfigMap", objs[0].GetKind())
}
