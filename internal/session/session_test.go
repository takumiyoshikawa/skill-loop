package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveRepoRoot(t *testing.T) {
	t.Run("returns current directory when not in git repo", func(t *testing.T) {
		tempDir := t.TempDir()
		root, err := ResolveRepoRoot(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tempDir {
			t.Errorf("expected %s, got %s", tempDir, root)
		}
	})
}

func TestSessionsRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoRoot := "/tmp/test-repo"
	expected := filepath.Join(home, ".local", "share", "skill-loop")
	got := SessionsRoot(repoRoot)
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestNew(t *testing.T) {
	t.Run("creates session with valid parameters", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta, err := New(tempDir, tempDir, "nightly-review", "test-skill", "claude", command, 10*time.Second, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.ID == "" {
			t.Error("expected non-empty ID")
		}
		if meta.Skill != "test-skill" {
			t.Errorf("expected skill 'test-skill', got %s", meta.Skill)
		}
		if meta.Runtime != "claude" {
			t.Errorf("expected runtime 'claude', got %s", meta.Runtime)
		}
		if meta.WorkflowName != "nightly-review" {
			t.Errorf("expected workflow 'nightly-review', got %s", meta.WorkflowName)
		}
		if meta.StorageName == "" {
			t.Error("expected non-empty storage name")
		}
		if meta.Status != StatusPending {
			t.Errorf("expected status %s, got %s", StatusPending, meta.Status)
		}
		if meta.IdleTimeoutSeconds != 10 {
			t.Errorf("expected idle timeout 10, got %d", meta.IdleTimeoutSeconds)
		}
		if meta.MaxRestarts != 2 {
			t.Errorf("expected max restarts 2, got %d", meta.MaxRestarts)
		}
	})

	t.Run("returns error when command is empty", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		_, err := New(tempDir, tempDir, "nightly-review", "test-skill", "claude", []string{}, 10*time.Second, 2)
		if err == nil {
			t.Error("expected error for empty command")
		}
	})

	t.Run("applies default idle timeout when zero", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta, err := New(tempDir, tempDir, "nightly-review", "test-skill", "claude", command, 0, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.IdleTimeoutSeconds != DefaultIdleTimeoutSeconds {
			t.Errorf("expected default idle timeout %d, got %d", DefaultIdleTimeoutSeconds, meta.IdleTimeoutSeconds)
		}
	})

	t.Run("applies default max restarts when negative", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta, err := New(tempDir, tempDir, "nightly-review", "test-skill", "claude", command, 10*time.Second, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.MaxRestarts != DefaultMaxRestarts {
			t.Errorf("expected default max restarts %d, got %d", DefaultMaxRestarts, meta.MaxRestarts)
		}
	})
}

func TestSaveAndLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tempDir := t.TempDir()
	command := []string{"echo", "hello"}

	meta, err := New(tempDir, tempDir, "nightly-review", "test-skill", "claude", command, 10*time.Second, 2)
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	loaded, err := LoadByID(tempDir, meta.ID)
	if err != nil {
		t.Fatalf("unexpected error loading session: %v", err)
	}

	if loaded.ID != meta.ID {
		t.Errorf("expected ID %s, got %s", meta.ID, loaded.ID)
	}
	if loaded.Skill != meta.Skill {
		t.Errorf("expected skill %s, got %s", meta.Skill, loaded.Skill)
	}
	if loaded.Status != meta.Status {
		t.Errorf("expected status %s, got %s", meta.Status, loaded.Status)
	}
}

func TestList(t *testing.T) {
	t.Run("returns empty list when no sessions exist", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		metas, err := List(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(metas) != 0 {
			t.Errorf("expected empty list, got %d sessions", len(metas))
		}
	})

	t.Run("returns sessions sorted by start time", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta1, err := New(tempDir, tempDir, "workflow-a", "skill-1", "claude", command, 10*time.Second, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		time.Sleep(10 * time.Millisecond)

		meta2, err := New(tempDir, tempDir, "workflow-b", "skill-2", "claude", command, 10*time.Second, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		metas, err := List(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(metas) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(metas))
		}

		// Should be sorted newest first
		if metas[0].ID != meta2.ID {
			t.Errorf("expected first session to be %s, got %s", meta2.ID, metas[0].ID)
		}
		if metas[1].ID != meta1.ID {
			t.Errorf("expected second session to be %s, got %s", meta1.ID, metas[1].ID)
		}
	})

	t.Run("filters sessions by repo root", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		repoOne := t.TempDir()
		repoTwo := t.TempDir()
		command := []string{"echo", "hello"}

		meta1, err := New(repoOne, repoOne, "workflow-a", "skill-1", "claude", command, 10*time.Second, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := New(repoTwo, repoTwo, "workflow-b", "skill-2", "claude", command, 10*time.Second, 2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		metas, err := List(repoOne)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(metas) != 1 {
			t.Fatalf("expected 1 session, got %d", len(metas))
		}
		if metas[0].ID != meta1.ID {
			t.Fatalf("expected session %s, got %s", meta1.ID, metas[0].ID)
		}
	})
}

func TestDeleteByID(t *testing.T) {
	t.Run("deletes existing session directory", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}
		meta, err := New(tempDir, tempDir, "nightly-review", "skill-1", "claude", command, 10*time.Second, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sessionDir := filepath.Dir(meta.ScriptPath)
		if _, err := os.Stat(sessionDir); err != nil {
			t.Fatalf("expected session directory to exist: %v", err)
		}

		if err := DeleteByID(tempDir, meta.ID); err != nil {
			t.Fatalf("DeleteByID() error: %v", err)
		}

		if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
			t.Fatalf("expected session directory to be deleted, got err=%v", err)
		}
	})

	t.Run("rejects empty session id", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		tempDir := t.TempDir()
		if err := DeleteByID(tempDir, ""); err == nil {
			t.Fatal("DeleteByID() should return error for empty id")
		}
	})
}

func TestReadExitCode(t *testing.T) {
	t.Run("returns false when file does not exist", func(t *testing.T) {
		_, hasCode, err := ReadExitCode("/nonexistent/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasCode {
			t.Error("expected hasCode to be false")
		}
	})

	t.Run("reads exit code from file", func(t *testing.T) {
		tempFile := filepath.Join(t.TempDir(), "exit.code")
		if err := os.WriteFile(tempFile, []byte("42\n"), 0o600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		code, hasCode, err := ReadExitCode(tempFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasCode {
			t.Error("expected hasCode to be true")
		}
		if code != 42 {
			t.Errorf("expected code 42, got %d", code)
		}
	})

	t.Run("returns false for empty file", func(t *testing.T) {
		tempFile := filepath.Join(t.TempDir(), "exit.code")
		if err := os.WriteFile(tempFile, []byte(""), 0o600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, hasCode, err := ReadExitCode(tempFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasCode {
			t.Error("expected hasCode to be false for empty file")
		}
	})
}

func TestReconcileKeepsScheduledStatusWhenTmuxSessionExists(t *testing.T) {
	binDir := t.TempDir()
	tmuxPath := filepath.Join(binDir, "tmux")
	if err := os.WriteFile(tmuxPath, []byte(`#!/bin/sh
if [ "$1" = "has-session" ]; then
  exit 0
fi
exit 0
	`), 0o600); err != nil {
		t.Fatalf("failed to write fake tmux: %v", err)
	}
	//nolint:gosec // Test helper script must be executable to stand in for tmux on PATH.
	if err := os.Chmod(tmuxPath, 0o755); err != nil {
		t.Fatalf("failed to write fake tmux: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	tempDir := t.TempDir()
	now := time.Now().UTC()
	nextRun := now.Add(time.Hour)
	meta := &Metadata{
		ID:           "scheduled-session",
		RepoRoot:     tempDir,
		ScriptPath:   filepath.Join(tempDir, "run.sh"),
		ExitCodePath: filepath.Join(tempDir, "exit.code"),
		StdoutPath:   filepath.Join(tempDir, "stdout.log"),
		StderrPath:   filepath.Join(tempDir, "stderr.log"),
		TmuxSession:  "skill-loop-scheduled-session",
		Status:       StatusScheduled,
		StartedAt:    now,
		LastOutputAt: now,
		NextRun:      &nextRun,
		Schedule:     "0 9 * * *",
	}

	if err := Reconcile(meta); err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}

	if meta.Status != StatusScheduled {
		t.Fatalf("status = %s, want %s", meta.Status, StatusScheduled)
	}
}

func TestReconcileKeepsBlockedStatusWhenExitCodeExists(t *testing.T) {
	tempDir := t.TempDir()
	exitCodePath := filepath.Join(tempDir, "exit.code")
	if err := os.WriteFile(exitCodePath, []byte("0\n"), 0o600); err != nil {
		t.Fatalf("failed to write exit code: %v", err)
	}

	meta := &Metadata{
		ID:           "blocked-session",
		RepoRoot:     tempDir,
		ScriptPath:   filepath.Join(tempDir, "run.sh"),
		ExitCodePath: exitCodePath,
		StdoutPath:   filepath.Join(tempDir, "stdout.log"),
		StderrPath:   filepath.Join(tempDir, "stderr.log"),
		TmuxSession:  "skill-loop-blocked-session",
		Status:       StatusBlocked,
		BlockReason:  "waiting for human approval",
		ResumeSkill:  "apply-feedback",
		ResumePrompt: "base handoff",
		StartedAt:    time.Now().UTC(),
		LastOutputAt: time.Now().UTC(),
	}

	if err := Reconcile(meta); err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if meta.Status != StatusBlocked {
		t.Fatalf("status = %s, want %s", meta.Status, StatusBlocked)
	}
	if meta.EndedAt == nil {
		t.Fatal("expected blocked reconcile to set ended_at")
	}
}

func TestBuildResumeCommand(t *testing.T) {
	meta := &Metadata{
		ID:          "blocked-session",
		Status:      StatusBlocked,
		ConfigPath:  "/repo/skill-loop.yml",
		ResumeSkill: "apply-feedback",
		ResumePrompt: strings.Join([]string{
			"Previous skill: review",
			"",
			"Previous skill stdout:",
			"needs a human",
		}, "\n"),
		Command: []string{
			"env",
			"SKILL_LOOP_RUN_CHILD=1",
			"/usr/local/bin/skill-loop",
			"run",
			"/repo/original.yml",
			"--max-iterations",
			"5",
			"--prompt",
			"old prompt",
			"--entrypoint",
			"review",
		},
	}

	command, err := BuildResumeCommand(meta, "Please continue")
	if err != nil {
		t.Fatalf("BuildResumeCommand() error: %v", err)
	}

	got := strings.Join(command, "\n")
	for _, want := range []string{
		"/repo/skill-loop.yml",
		"--max-iterations",
		"5",
		"--entrypoint",
		"apply-feedback",
		"Human input:\nPlease continue",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildResumeCommand() missing %q\nfull command:\n%s", want, got)
		}
	}
}

func TestBuildResumeCommandWithGoRunTemplate(t *testing.T) {
	meta := &Metadata{
		ID:          "blocked-session",
		Status:      StatusBlocked,
		ConfigPath:  "/repo/e2e/resume.yml",
		ResumeSkill: "pass-through",
		ResumePrompt: strings.Join([]string{
			"Previous skill: pass-through",
			"",
			"Previous skill stdout:",
			"<NEEDS_HUMAN>",
		}, "\n"),
		Command: []string{
			"env",
			"SKILL_LOOP_RUN_CHILD=1",
			"go",
			"run",
			"./cmd/skill-loop",
			"run",
			"/repo/old.yml",
			"--max-iterations",
			"2",
		},
	}

	command, err := BuildResumeCommand(meta, "ok")
	if err != nil {
		t.Fatalf("BuildResumeCommand() error: %v", err)
	}
	if strings.Join(command[:5], " ") != "env SKILL_LOOP_RUN_CHILD=1 go run ./cmd/skill-loop" {
		t.Fatalf("unexpected command prefix: %q", strings.Join(command, " "))
	}
	if command[5] != "run" || command[6] != "/repo/e2e/resume.yml" {
		t.Fatalf("unexpected run/config segment: %v", command)
	}
}

func TestUpdateCommandPrefersConfigDirectory(t *testing.T) {
	tempDir := t.TempDir()
	meta := &Metadata{
		ConfigPath:   filepath.Join(tempDir, "e2e", "resume.yml"),
		WorkingDir:   tempDir,
		ScriptPath:   filepath.Join(tempDir, "run.sh"),
		StdoutPath:   filepath.Join(tempDir, "stdout.log"),
		StderrPath:   filepath.Join(tempDir, "stderr.log"),
		ExitCodePath: filepath.Join(tempDir, "exit.code"),
		Command:      []string{"echo", "hello"},
	}
	if err := os.MkdirAll(filepath.Dir(meta.ConfigPath), 0o750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if err := UpdateCommand(meta, []string{"echo", "updated"}); err != nil {
		t.Fatalf("UpdateCommand() error: %v", err)
	}

	if meta.WorkingDir != filepath.Dir(meta.ConfigPath) {
		t.Fatalf("workingDir = %q, want %q", meta.WorkingDir, filepath.Dir(meta.ConfigPath))
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"don't", `'don'"'"'t'`},
		{"it's a test", `'it'"'"'s a test'`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewID(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	id, err := newID(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ID should start with timestamp
	expectedPrefix := "20240115T103045Z-"
	if len(id) < len(expectedPrefix)+8 {
		t.Errorf("ID too short: %s", id)
	}
	if id[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected ID to start with %s, got %s", expectedPrefix, id)
	}
}
