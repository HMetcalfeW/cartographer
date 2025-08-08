package cmd_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/HMetcalfeW/cartographer/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCommand_NoInputOrChart(t *testing.T) {
	// Reset the root command arguments.
	root := cmd.RootCmd
	root.SetArgs([]string{"analyze"})

	// Capture output.
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	// Execute the command and expect an error.
	err := root.Execute()
	require.Error(t, err, "expected error when no input or chart is provided")
		assert.Contains(t, err.Error(), "error: No input file or chart provided. Please specify either --input or --chart.")
}

func TestAnalyzeCommand_WithInput(t *testing.T) {
	// Create a temporary YAML file with one Kubernetes document.
	yamlContent := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
`
	tmpfile, err := os.CreateTemp("", "analyze-test-*.yaml")
	require.NoError(t, err, "failed to create temp file")
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}()

	_, err = tmpfile.Write([]byte(yamlContent))
	require.NoError(t, err, "failed to write YAML content")
	err = tmpfile.Close()
	require.NoError(t, err, "failed to close temp file")

	// Set the command arguments to use the temporary file.
	root := cmd.RootCmd
	root.SetArgs([]string{"analyze", "--input", tmpfile.Name()})

	// Capture output.
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	// Execute the command.
	err = root.Execute()
	require.NoError(t, err, "expected no error when input file is provided")
}
