package dependency

import (
	"bytes"
	"fmt"
	"os/exec"
)

// RenderImage generates DOT from the dependency map and pipes it through
// the GraphViz dot command to produce image output in the given format
// ("png" or "svg").
//
// Returns the raw image bytes or an error if GraphViz is not installed
// or the rendering fails.
func RenderImage(deps map[string][]Edge, format string) ([]byte, error) {
	dotPath, err := exec.LookPath("dot")
	if err != nil {
		return nil, fmt.Errorf(
			"graphviz 'dot' command not found on PATH: %w\n\n"+
				"Install GraphViz:\n"+
				"  macOS:   brew install graphviz\n"+
				"  Ubuntu:  sudo apt-get install graphviz\n"+
				"  Fedora:  sudo dnf install graphviz",
			err,
		)
	}

	dotContent := GenerateDOT(deps)

	cmd := exec.Command(dotPath, "-T"+format)
	cmd.Stdin = bytes.NewReader([]byte(dotContent))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("graphviz rendering failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}
