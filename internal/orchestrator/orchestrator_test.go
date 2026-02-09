package orchestrator

import (
	"fmt"
	"testing"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/executor"
)

type mockExecutor struct {
	calls   []mockCall
	callIdx int
}

type mockCall struct {
	result *executor.SkillResult
	err    error
}

func (m *mockExecutor) ExecuteSkill(name string, model string, prevSummary string) (*executor.SkillResult, error) {
	if m.callIdx >= len(m.calls) {
		return nil, fmt.Errorf("unexpected call #%d to ExecuteSkill(%q)", m.callIdx, name)
	}
	call := m.calls[m.callIdx]
	m.callIdx++
	return call.result, call.err
}

func TestRunSingleSkillDone(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: "review",
		Skills: map[string]config.Skill{
			"review": {
				Next: []config.Route{
					{Skill: "<DONE>"},
				},
			},
		},
	}

	mock := &mockExecutor{
		calls: []mockCall{
			{result: &executor.SkillResult{Summary: "All good"}},
		},
	}

	err := RunWith(cfg, 10, "", mock)
	if err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}

	if mock.callIdx != 1 {
		t.Errorf("expected 1 call, got %d", mock.callIdx)
	}
}

func TestRunSkillChain(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: "impl",
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{Skill: "review"}},
			},
			"review": {
				Next: []config.Route{
					{When: "<REVIEW_OK>", Skill: "deploy"},
					{Skill: "impl"},
				},
			},
			"deploy": {
				Next: []config.Route{{Skill: "<DONE>"}},
			},
		},
	}

	mock := &mockExecutor{
		calls: []mockCall{
			{result: &executor.SkillResult{Summary: "implemented feature"}},
			{result: &executor.SkillResult{Summary: "needs work"}},
			{result: &executor.SkillResult{Summary: "re-implemented"}},
			{result: &executor.SkillResult{Summary: "<REVIEW_OK>"}},
			{result: &executor.SkillResult{Summary: "deployed"}},
		},
	}

	err := RunWith(cfg, 10, "", mock)
	if err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}

	if mock.callIdx != 5 {
		t.Errorf("expected 5 calls, got %d", mock.callIdx)
	}
}

func TestRunLoopBack(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: "impl",
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{Skill: "review"}},
			},
			"review": {
				Next: []config.Route{
					{When: "<REVIEW_OK>", Skill: "<DONE>"},
					{Skill: "impl"},
				},
			},
		},
	}

	mock := &mockExecutor{
		calls: []mockCall{
			{result: &executor.SkillResult{Summary: "done impl"}},
			{result: &executor.SkillResult{Summary: "needs fix"}}, // no match → impl
			{result: &executor.SkillResult{Summary: "re-impl"}},
			{result: &executor.SkillResult{Summary: "<REVIEW_OK>"}}, // match → <DONE>
		},
	}

	err := RunWith(cfg, 10, "", mock)
	if err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}

	if mock.callIdx != 4 {
		t.Errorf("expected 4 calls, got %d", mock.callIdx)
	}
}

func TestRunSkillExecutionError(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: "review",
		Skills: map[string]config.Skill{
			"review": {
				Next: []config.Route{{Skill: "<DONE>"}},
			},
		},
	}

	mock := &mockExecutor{
		calls: []mockCall{
			{result: nil, err: fmt.Errorf("claude not found")},
		},
	}

	err := RunWith(cfg, 10, "", mock)
	if err == nil {
		t.Error("RunWith() should return error when executor fails")
	}
}

func TestRunMaxIterationsReached(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: "impl",
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{Skill: "review"}},
			},
			"review": {
				Next: []config.Route{{Skill: "impl"}},
			},
		},
	}

	mock := &mockExecutor{
		calls: []mockCall{
			{result: &executor.SkillResult{Summary: "impl"}},
			{result: &executor.SkillResult{Summary: "review"}},
			{result: &executor.SkillResult{Summary: "impl"}},
		},
	}

	err := RunWith(cfg, 3, "", mock)
	if err == nil {
		t.Error("RunWith() should return error when max iterations reached")
	}
}

func TestRunDefaultMaxIterations(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: "review",
		Skills: map[string]config.Skill{
			"review": {
				Next: []config.Route{{Skill: "<DONE>"}},
			},
		},
	}

	mock := &mockExecutor{
		calls: []mockCall{
			{result: &executor.SkillResult{Summary: "Done"}},
		},
	}

	err := RunWith(cfg, 0, "", mock)
	if err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}
}

func TestRunNoRoutes(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: "review",
		Skills: map[string]config.Skill{
			"review": {
				Next: nil,
			},
		},
	}

	mock := &mockExecutor{
		calls: []mockCall{
			{result: &executor.SkillResult{Summary: "Done"}},
		},
	}

	err := RunWith(cfg, 10, "", mock)
	if err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}
}

func TestResolveNext(t *testing.T) {
	routes := []config.Route{
		{When: "<REVIEW_OK>", Skill: "<DONE>"},
		{Skill: "impl"},
	}

	// Match first route
	got := resolveNext(routes, "all good <REVIEW_OK>")
	if got != "<DONE>" {
		t.Errorf("resolveNext() = %q, want %q", got, "<DONE>")
	}

	// Fall through to default
	got = resolveNext(routes, "needs more work")
	if got != "impl" {
		t.Errorf("resolveNext() = %q, want %q", got, "impl")
	}

	// Empty routes → <DONE>
	got = resolveNext(nil, "anything")
	if got != "<DONE>" {
		t.Errorf("resolveNext(nil) = %q, want %q", got, "<DONE>")
	}
}
