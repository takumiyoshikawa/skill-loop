package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDetachedBinaryUsesInstalledBinaryForGoRun(t *testing.T) {
	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "skill-loop")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	//nolint:gosec // Test helper must be executable to stand in for the installed binary on PATH.
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := resolveDetachedBinary("/var/folders/tmp/go-build1234/b001/exe/skill-loop")
	if err != nil {
		t.Fatalf("resolveDetachedBinary() error: %v", err)
	}
	if got != binaryPath {
		t.Fatalf("resolveDetachedBinary() = %q, want %q", got, binaryPath)
	}
}

func TestResolveDetachedBinaryErrorsWhenInstalledBinaryMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := resolveDetachedBinary("/var/folders/tmp/go-build1234/b001/exe/skill-loop")
	if err == nil || !strings.Contains(err.Error(), "installed skill-loop binary") {
		t.Fatalf("resolveDetachedBinary() error = %v, want installed binary error", err)
	}
}
