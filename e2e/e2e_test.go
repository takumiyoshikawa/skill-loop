package e2e

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestE2E(t *testing.T) {
	var buf bytes.Buffer
	cmd := exec.Command("go", "run", "../cmd/skill-loop", "run")
	cmd.Dir = "."
	cmd.Stdout = io.MultiWriter(&buf, os.Stdout)
	cmd.Stderr = io.MultiWriter(&buf, os.Stderr)

	err := cmd.Run()
	if err != nil {
		t.Fatalf("skill-loop failed: %v\noutput:\n%s", err, buf.String())
	}

	output := buf.String()

	expectedLines := []string{
		"==> Running skill: 1-impl (iteration 1)",
		"Summary: <IMPL_DONE>",
		"==> Running skill: 2-review (iteration 2)",
		"Summary: 2-review reviewed",
		"==> Running skill: 1-impl (iteration 3)",
		"Summary: <IMPL_DONE> You already reviewed.",
		"==> Running skill: 2-review (iteration 4)",
		"Summary: <REVIEW_OK>",
		"==> Loop finished.",
	}

	for _, line := range expectedLines {
		if !strings.Contains(output, line) {
			t.Errorf("output missing expected line: %q\nfull output:\n%s", line, output)
		}
	}
}
