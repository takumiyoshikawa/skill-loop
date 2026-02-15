package session

import (
	"os"
	"path/filepath"
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
	repoRoot := "/tmp/test-repo"
	expected := filepath.Join(repoRoot, ".skill-loop", "sessions")
	got := SessionsRoot(repoRoot)
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestNew(t *testing.T) {
	t.Run("creates session with valid parameters", func(t *testing.T) {
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta, err := New(tempDir, tempDir, "test-skill", "claude", command, 10*time.Second, 2)
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
		tempDir := t.TempDir()
		_, err := New(tempDir, tempDir, "test-skill", "claude", []string{}, 10*time.Second, 2)
		if err == nil {
			t.Error("expected error for empty command")
		}
	})

	t.Run("applies default idle timeout when zero", func(t *testing.T) {
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta, err := New(tempDir, tempDir, "test-skill", "claude", command, 0, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.IdleTimeoutSeconds != DefaultIdleTimeoutSeconds {
			t.Errorf("expected default idle timeout %d, got %d", DefaultIdleTimeoutSeconds, meta.IdleTimeoutSeconds)
		}
	})

	t.Run("applies default max restarts when negative", func(t *testing.T) {
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta, err := New(tempDir, tempDir, "test-skill", "claude", command, 10*time.Second, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.MaxRestarts != DefaultMaxRestarts {
			t.Errorf("expected default max restarts %d, got %d", DefaultMaxRestarts, meta.MaxRestarts)
		}
	})
}

func TestSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	command := []string{"echo", "hello"}

	meta, err := New(tempDir, tempDir, "test-skill", "claude", command, 10*time.Second, 2)
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
		tempDir := t.TempDir()
		command := []string{"echo", "hello"}

		meta1, err := New(tempDir, tempDir, "skill-1", "claude", command, 10*time.Second, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		time.Sleep(10 * time.Millisecond)

		meta2, err := New(tempDir, tempDir, "skill-2", "claude", command, 10*time.Second, 2)
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
