package version_test

import (
	"bytes"
	"testing"

	"github.com/HMetcalfeW/cartographer/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	root := cmd.RootCmd
	root.SetArgs([]string{"version"})

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	err := root.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "cartographer")
	assert.Contains(t, output, "commit:")
	assert.Contains(t, output, "built:")
}
