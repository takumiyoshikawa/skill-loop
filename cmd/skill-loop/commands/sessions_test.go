package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takumiyoshikawa/skill-loop/internal/session"
)

func TestResolveLogSelection(t *testing.T) {
	tests := []struct {
		name       string
		stdout     bool
		stderr     bool
		wantStdout bool
		wantStderr bool
	}{
		{name: "default to both", wantStdout: true, wantStderr: true},
		{name: "stdout only", stdout: true, wantStdout: true},
		{name: "stderr only", stderr: true, wantStderr: true},
		{name: "explicit both", stdout: true, stderr: true, wantStdout: true, wantStderr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStdout, gotStderr := resolveLogSelection(tt.stdout, tt.stderr)
			if gotStdout != tt.wantStdout || gotStderr != tt.wantStderr {
				t.Fatalf("resolveLogSelection() = (%t, %t), want (%t, %t)", gotStdout, gotStderr, tt.wantStdout, tt.wantStderr)
			}
		})
	}
}

func TestTailLines(t *testing.T) {
	input := "one\ntwo\nthree\nfour\n"
	got := tailLines(input, 2)
	want := "three\nfour\n"
	if got != want {
		t.Fatalf("tailLines() = %q, want %q", got, want)
	}
}

func TestReadLogFileMissingReturnsEmpty(t *testing.T) {
	got, err := readLogFile(filepath.Join(t.TempDir(), "missing.log"), 0)
	if err != nil {
		t.Fatalf("readLogFile() error: %v", err)
	}
	if got != "" {
		t.Fatalf("readLogFile() = %q, want empty", got)
	}
}

func TestFormatSessionDetailsIncludesPathsAndError(t *testing.T) {
	now := time.Date(2026, 3, 7, 3, 4, 5, 0, time.UTC)
	ended := now.Add(2 * time.Minute)
	meta := &session.Metadata{
		ID:           "session-123",
		Status:       session.StatusBlocked,
		StartedAt:    now,
		LastOutputAt: now.Add(time.Minute),
		EndedAt:      &ended,
		ScriptPath:   "/tmp/session-123/run.sh",
		StdoutPath:   "/tmp/session-123/stdout.log",
		StderrPath:   "/tmp/session-123/stderr.log",
		WorkingDir:   "/repo",
		BlockReason:  "waiting for review",
		ResumeSkill:  "apply-feedback",
	}

	got := formatSessionDetails(meta)

	for _, want := range []string{
		"ID: session-123",
		"Status: blocked",
		"Session: /tmp/session-123",
		"Stdout: /tmp/session-123/stdout.log",
		"Stderr: /tmp/session-123/stderr.log",
		"Working dir: /repo",
		"Block reason: waiting for review",
		"Resume skill: apply-feedback",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatSessionDetails() missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestPrintLogSectionEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdout.log")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var b strings.Builder
	if err := printLogSection(&b, "stdout", path, 0); err != nil {
		t.Fatalf("printLogSection() error: %v", err)
	}

	got := b.String()
	if !strings.Contains(got, "==> stdout ("+path+")") {
		t.Fatalf("printLogSection() missing header:\n%s", got)
	}
	if !strings.Contains(got, "(empty)") {
		t.Fatalf("printLogSection() missing empty marker:\n%s", got)
	}
}

func TestSessionsShowArgs(t *testing.T) {
	t.Run("show rejects positional args", func(t *testing.T) {
		cmd := newSessionsShowCmd()
		if err := cmd.Args(cmd, nil); err != nil {
			t.Fatalf("Args() error: %v", err)
		}
		if err := cmd.Args(cmd, []string{"session-123"}); err == nil {
			t.Fatal("Args() should reject positional args")
		}
	})
}
