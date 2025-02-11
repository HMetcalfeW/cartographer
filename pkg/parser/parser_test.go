package parser_test

import (
	"os"
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/parser"
)

func TestParseYAMLFile(t *testing.T) {
	// Create a temporary YAML file with two Kubernetes documents.
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

	tmpfile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Defer removal and check the error returned by os.Remove.
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}()

	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close the temp file: %v", err)
	}

	// Invoke the parser.
	objs, err := parser.ParseYAMLFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseYAMLFile returned error: %v", err)
	}

	// We expect two objects.
	if len(objs) != 2 {
		t.Fatalf("Expected 2 objects, got %d", len(objs))
	}

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

	if !foundPod {
		t.Error("Expected to find a Pod object, but did not")
	}
	if !foundDeployment {
		t.Error("Expected to find a Deployment object, but did not")
	}
}
