package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/HMetcalfeW/cartographer/cmd"
	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multiResourceYAML provides a realistic manifest with edges for integration tests.
const multiResourceYAML = `
apiVersion: v1
kind: Secret
metadata:
  name: db-creds
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
      containers:
        - name: web
          image: nginx
          env:
            - name: DB_PASS
              valueFrom:
                secretKeyRef:
                  name: db-creds
                  key: password
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
`

// writeTestInput creates a temp YAML file and returns its path. The caller
// must defer removal.
func writeTestInput(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "analyze-test-*.yaml")
	require.NoError(t, err)
	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

// tempOutputPath returns a path for a temp output file. The caller should
// defer removal.
func tempOutputPath(t *testing.T, pattern string) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

func graphvizAvailable() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}

func TestAnalyzeCommand_NoInputOrChart(t *testing.T) {
	root := cmd.RootCmd
	root.SetArgs([]string{"analyze"})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.Error(t, err, "expected error when no input or chart is provided")
	assert.Contains(t, err.Error(), "no input file or chart provided")
}

func TestAnalyzeCommand_WithInput(t *testing.T) {
	inputPath := writeTestInput(t, `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
`)

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err, "expected no error when input file is provided")
}

func TestAnalyzeCommand_MermaidStdout(t *testing.T) {
	inputPath := writeTestInput(t, multiResourceYAML)

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "mermaid"})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "-->")
}

func TestAnalyzeCommand_MermaidFile(t *testing.T) {
	inputPath := writeTestInput(t, multiResourceYAML)
	outputPath := tempOutputPath(t, "mermaid-*.md")

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "mermaid", "--output-file", outputPath})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "graph LR")
	assert.Contains(t, string(data), "-->")
}

func TestAnalyzeCommand_JSONStdout(t *testing.T) {
	inputPath := writeTestInput(t, multiResourceYAML)

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "json", "--output-file", ""})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"nodes"`)
	assert.Contains(t, output, `"edges"`)

	// Verify it's valid JSON.
	var graph dependency.JSONGraph
	require.NoError(t, json.Unmarshal([]byte(output), &graph), "output must be valid JSON")
	assert.Greater(t, len(graph.Nodes), 0, "should have nodes")
	assert.Greater(t, len(graph.Edges), 0, "should have edges")
}

func TestAnalyzeCommand_JSONFile(t *testing.T) {
	inputPath := writeTestInput(t, multiResourceYAML)
	outputPath := tempOutputPath(t, "output-*.json")

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "json", "--output-file", outputPath})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var graph dependency.JSONGraph
	require.NoError(t, json.Unmarshal(data, &graph), "file must contain valid JSON")
	assert.Greater(t, len(graph.Nodes), 0)
}

func TestAnalyzeCommand_PNGFile(t *testing.T) {
	if !graphvizAvailable() {
		t.Skip("graphviz not installed, skipping PNG integration test")
	}
	inputPath := writeTestInput(t, multiResourceYAML)
	outputPath := tempOutputPath(t, "output-*.png")

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "png", "--output-file", outputPath})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.True(t, len(data) > 4, "PNG file should not be empty")
	// PNG magic bytes: 0x89 0x50 0x4E 0x47
	assert.Equal(t, byte(0x89), data[0])
	assert.Equal(t, byte(0x50), data[1])
	assert.Equal(t, byte(0x4E), data[2])
	assert.Equal(t, byte(0x47), data[3])
}

func TestAnalyzeCommand_SVGFile(t *testing.T) {
	if !graphvizAvailable() {
		t.Skip("graphviz not installed, skipping SVG integration test")
	}
	inputPath := writeTestInput(t, multiResourceYAML)
	outputPath := tempOutputPath(t, "output-*.svg")

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "svg", "--output-file", outputPath})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<svg")
	assert.Contains(t, string(data), "</svg>")
}

// matchExpressionsYAML provides a manifest exercising matchExpressions for CLI integration tests.
const matchExpressionsYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app.kubernetes.io/name: myapp
    app.kubernetes.io/component: frontend
    env: prod
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  labels:
    app.kubernetes.io/name: myapp
    app.kubernetes.io/component: backend
    env: prod
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  labels:
    app.kubernetes.io/name: myapp
    app.kubernetes.io/component: worker
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
      app.kubernetes.io/component: backend
    matchExpressions:
      - key: env
        operator: NotIn
        values: [staging]
`

func TestAnalyzeCommand_MatchExpressions(t *testing.T) {
	inputPath := writeTestInput(t, matchExpressionsYAML)

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "json", "--output-file", ""})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	output := buf.String()
	var graph dependency.JSONGraph
	require.NoError(t, json.Unmarshal([]byte(output), &graph), "output must be valid JSON")

	// Build a set of edges for easy lookup
	edgeSet := make(map[string]string)
	for _, e := range graph.Edges {
		edgeSet[e.From+"|"+e.To] = e.Reason
	}

	// NetworkPolicy/prod-only should have podSelector edges to web and api
	assert.Equal(t, "podSelector", edgeSet["NetworkPolicy/prod-only|Deployment/web"])
	assert.Equal(t, "podSelector", edgeSet["NetworkPolicy/prod-only|Deployment/api"])
	_, hasWorker := edgeSet["NetworkPolicy/prod-only|Deployment/worker"]
	assert.False(t, hasWorker, "worker is env=staging, should not match In [prod]")

	// PDB/backend-pdb should match only api (component=backend AND env NotIn [staging])
	assert.Equal(t, "pdbSelector", edgeSet["PodDisruptionBudget/backend-pdb|Deployment/api"])
	_, hasWeb := edgeSet["PodDisruptionBudget/backend-pdb|Deployment/web"]
	assert.False(t, hasWeb, "web is frontend, not backend")
	_, hasWorkerPdb := edgeSet["PodDisruptionBudget/backend-pdb|Deployment/worker"]
	assert.False(t, hasWorkerPdb, "worker is staging, excluded by NotIn")
}

func TestAnalyzeCommand_MatchExpressions_MermaidStdout(t *testing.T) {
	inputPath := writeTestInput(t, matchExpressionsYAML)

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "mermaid"})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")
	// Verify matchExpressions-derived edges appear in the Mermaid output
	assert.Contains(t, output, "NetworkPolicy/prod-only")
	assert.Contains(t, output, "Deployment/web")
	assert.Contains(t, output, "PodDisruptionBudget/backend-pdb")
	assert.Contains(t, output, "-->")
}

func TestAnalyzeCommand_MatchExpressions_DOTStdout(t *testing.T) {
	inputPath := writeTestInput(t, matchExpressionsYAML)

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "dot"})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "digraph G {")
	assert.Contains(t, output, `"NetworkPolicy/prod-only" -> "Deployment/web"`)
	assert.Contains(t, output, `"NetworkPolicy/prod-only" -> "Deployment/api"`)
	assert.Contains(t, output, `"PodDisruptionBudget/backend-pdb" -> "Deployment/api"`)
	// worker should NOT appear as a target of prod-only
	assert.NotContains(t, output, `"NetworkPolicy/prod-only" -> "Deployment/worker"`)
}

func TestAnalyzeCommand_MatchExpressions_PNGFile(t *testing.T) {
	if !graphvizAvailable() {
		t.Skip("graphviz not installed, skipping PNG integration test")
	}
	inputPath := writeTestInput(t, matchExpressionsYAML)
	outputPath := tempOutputPath(t, "matchexpr-*.png")

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "png", "--output-file", outputPath})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.True(t, len(data) > 4, "PNG file should not be empty")
	// PNG magic bytes
	assert.Equal(t, byte(0x89), data[0])
	assert.Equal(t, byte(0x50), data[1])
	assert.Equal(t, byte(0x4E), data[2])
	assert.Equal(t, byte(0x47), data[3])
}

func TestAnalyzeCommand_MatchExpressions_SVGFile(t *testing.T) {
	if !graphvizAvailable() {
		t.Skip("graphviz not installed, skipping SVG integration test")
	}
	inputPath := writeTestInput(t, matchExpressionsYAML)
	outputPath := tempOutputPath(t, "matchexpr-*.svg")

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "svg", "--output-file", outputPath})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<svg")
	assert.Contains(t, string(data), "</svg>")
}

func TestAnalyzeCommand_PNGRequiresOutputFile(t *testing.T) {
	inputPath := writeTestInput(t, multiResourceYAML)

	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", inputPath, "--output-format", "png", "--output-file", ""})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--output-file is required")
}
