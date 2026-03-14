package sessionui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takumiyoshikawa/skill-loop/internal/session"
)

func TestListSessionsFiltersAndFormats(t *testing.T) {
	now := time.Date(2026, 3, 7, 8, 9, 10, 0, time.UTC)
	running := &session.Metadata{
		ID:              "run-1",
		WorkflowName:    "nightly-review",
		Skill:           "orchestrator",
		Runtime:         "skill-loop",
		RepoRoot:        "/repo",
		ConfigPath:      "/repo/skill-loop.yml",
		ScriptPath:      "/tmp/run-1/run.sh",
		StdoutPath:      "/tmp/run-1/stdout.log",
		StderrPath:      "/tmp/run-1/stderr.log",
		ExitCodePath:    "/tmp/run-1/exit.code",
		Status:          session.StatusRunning,
		StartedAt:       now,
		LastOutputAt:    now.Add(2 * time.Minute),
		LastSkillOutput: "latest stdout summary",
		Command:         []string{"skill-loop", "run"},
	}

	h := &handler{
		repoRoot: "/repo",
		store: sessionStore{
			list: func(repoRoot string) ([]*session.Metadata, error) {
				return []*session.Metadata{
					running,
					{ID: "skip-me", Skill: "other"},
				}, nil
			},
			reconcile: func(meta *session.Metadata) error { return nil },
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rec := httptest.NewRecorder()

	h.handleListSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got listResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.RepoRoot != "/repo" {
		t.Fatalf("repoRoot = %q, want /repo", got.RepoRoot)
	}
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(got.Sessions))
	}
	if got.Sessions[0].ConfigName != "nightly-review" {
		t.Fatalf("configName = %q, want nightly-review", got.Sessions[0].ConfigName)
	}
	if got.Sessions[0].WorkflowName != "nightly-review" {
		t.Fatalf("workflowName = %q, want nightly-review", got.Sessions[0].WorkflowName)
	}
	if !strings.Contains(got.Sessions[0].Detail, "last_output:") {
		t.Fatalf("detail = %q, want running summary", got.Sessions[0].Detail)
	}
	if got.Sessions[0].PreviousSummary != "latest stdout summary" {
		t.Fatalf("previousSummary = %q, want latest stdout summary", got.Sessions[0].PreviousSummary)
	}
}

func TestGetLogSupportsMissingFile(t *testing.T) {
	h := &handler{
		repoRoot: "/repo",
		store: sessionStore{
			load: func(repoRoot, id string) (*session.Metadata, error) {
				return &session.Metadata{
					ID:         id,
					Skill:      "orchestrator",
					StdoutPath: "/tmp/stdout.log",
				}, nil
			},
			reconcile: func(meta *session.Metadata) error { return nil },
			readFile: func(path string) ([]byte, error) {
				return nil, os.ErrNotExist
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/run-1/logs/stdout", nil)
	req.SetPathValue("id", "run-1")
	req.SetPathValue("stream", "stdout")
	rec := httptest.NewRecorder()

	h.handleGetLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got logResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Content != "" {
		t.Fatalf("content = %q, want empty", got.Content)
	}
}

func TestDeleteSessionRejectsRunningSession(t *testing.T) {
	h := &handler{
		repoRoot: "/repo",
		store: sessionStore{
			load: func(repoRoot, id string) (*session.Metadata, error) {
				return &session.Metadata{
					ID:     id,
					Skill:  "orchestrator",
					Status: session.StatusRunning,
				}, nil
			},
			reconcile: func(meta *session.Metadata) error { return nil },
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/run-1", nil)
	req.SetPathValue("id", "run-1")
	rec := httptest.NewRecorder()

	h.handleDeleteSession(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestResumeSession(t *testing.T) {
	meta := &session.Metadata{
		ID:          "blocked-1",
		Skill:       "orchestrator",
		Status:      session.StatusBlocked,
		BlockReason: "waiting for approval",
		ResumeSkill: "apply-feedback",
	}

	var gotPrompt string
	h := &handler{
		repoRoot: "/repo",
		store: sessionStore{
			load: func(repoRoot, id string) (*session.Metadata, error) {
				return meta, nil
			},
			reconcile: func(meta *session.Metadata) error { return nil },
			resume: func(meta *session.Metadata, prompt string) error {
				gotPrompt = prompt
				meta.Status = session.StatusRunning
				meta.BlockReason = ""
				meta.ResumeSkill = ""
				return nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/blocked-1/resume", strings.NewReader(`{"prompt":"ship it"}`))
	req.SetPathValue("id", "blocked-1")
	rec := httptest.NewRecorder()

	h.handleResumeSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotPrompt != "ship it" {
		t.Fatalf("prompt = %q, want ship it", gotPrompt)
	}
}

func TestExtractPreviousSummary(t *testing.T) {
	got := extractPreviousSummary(strings.Join([]string{
		"Previous skill: review",
		"Selected route: ask-human",
		"",
		"Previous skill stdout:",
		"<NEEDS_HUMAN>",
	}, "\n"))
	if got != "<NEEDS_HUMAN>" {
		t.Fatalf("summary = %q, want %q", got, "<NEEDS_HUMAN>")
	}
}

func TestPreviousSummaryPrefersLastSkillOutput(t *testing.T) {
	meta := &session.Metadata{
		LastSkillOutput: "latest",
		ResumePrompt:    "Previous skill stdout:\nstale",
	}
	if got := previousSummary(meta); got != "latest" {
		t.Fatalf("previousSummary() = %q, want latest", got)
	}
}

func TestPruneSessionsSkipsNonTerminalByDefault(t *testing.T) {
	done := &session.Metadata{ID: "done-1", Skill: "orchestrator", Status: session.StatusDone}
	idle := &session.Metadata{ID: "idle-1", Skill: "orchestrator", Status: session.StatusIdle}

	var deleted []string
	h := &handler{
		repoRoot: "/repo",
		store: sessionStore{
			list: func(repoRoot string) ([]*session.Metadata, error) {
				return []*session.Metadata{done, idle}, nil
			},
			reconcile: func(meta *session.Metadata) error { return nil },
			deleteByID: func(repoRoot, id string) error {
				deleted = append(deleted, id)
				return nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/prune", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.handlePruneSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(deleted) != 1 || deleted[0] != "done-1" {
		t.Fatalf("deleted = %v, want [done-1]", deleted)
	}
}

func TestTailLines(t *testing.T) {
	got := tailLines("one\ntwo\nthree\nfour\n", 2)
	want := "three\nfour\n"
	if got != want {
		t.Fatalf("tailLines() = %q, want %q", got, want)
	}
}
