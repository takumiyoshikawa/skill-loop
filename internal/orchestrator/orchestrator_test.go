package orchestrator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/executor"
)

type mockExecutor struct {
	skillCalls    []mockSkillCall
	routerCalls   []mockRouterCall
	skillCallIdx  int
	routerCallIdx int
	skillInputs   []string
	routerInputs  []string
}

type mockSkillCall struct {
	result *executor.SkillResult
	err    error
}

type mockRouterCall struct {
	result *executor.RouterDecision
	err    error
}

type mockObserver struct {
	iterations []int
	maxes      []int
	skills     []string
}

func (m *mockObserver) IterationStarted(iteration int, maxIterations int, skill string) {
	m.iterations = append(m.iterations, iteration)
	m.maxes = append(m.maxes, maxIterations)
	m.skills = append(m.skills, skill)
}

func (m *mockExecutor) ExecuteSkill(name string, agent config.Agent, input string, opts executor.ExecutionOptions) (*executor.SkillResult, error) {
	m.skillInputs = append(m.skillInputs, input)
	if m.skillCallIdx >= len(m.skillCalls) {
		return nil, fmt.Errorf("unexpected call #%d to ExecuteSkill(%q)", m.skillCallIdx, name)
	}
	call := m.skillCalls[m.skillCallIdx]
	m.skillCallIdx++
	return call.result, call.err
}

func (m *mockExecutor) RouteSkillOutput(skillName string, router config.Agent, output string, routes []config.Route, opts executor.ExecutionOptions) (*executor.RouterDecision, error) {
	m.routerInputs = append(m.routerInputs, output)
	if m.routerCallIdx >= len(m.routerCalls) {
		return nil, fmt.Errorf("unexpected call #%d to RouteSkillOutput(%q)", m.routerCallIdx, skillName)
	}
	call := m.routerCalls[m.routerCallIdx]
	m.routerCallIdx++
	return call.result, call.err
}

func TestRunSingleSkillDone(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "review",
		Skills: map[string]config.Skill{
			"review": {
				Next: []config.Route{{ID: "approve", Done: true}},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "All good"}},
		},
	}

	if err := RunWith(cfg, 10, "", "", mock); err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}
	if mock.skillCallIdx != 1 {
		t.Errorf("expected 1 skill call, got %d", mock.skillCallIdx)
	}
	if mock.routerCallIdx != 0 {
		t.Errorf("expected 0 router calls, got %d", mock.routerCallIdx)
	}
}

func TestRunSkillChainWithRouter(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "impl",
		Router:            config.Agent{Runtime: "codex", Model: "gpt-5.4"},
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{ID: "review", Skill: "review"}},
			},
			"review": {
				Next: []config.Route{
					{ID: "approve", Criteria: "looks good", Done: true},
					{ID: "rework", Criteria: "needs work", Skill: "impl"},
				},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "implemented feature"}},
			{result: &executor.SkillResult{Stdout: "needs more work"}},
			{result: &executor.SkillResult{Stdout: "reimplemented feature"}},
			{result: &executor.SkillResult{Stdout: "approved"}},
		},
		routerCalls: []mockRouterCall{
			{result: &executor.RouterDecision{Route: "rework", Reason: "Found a blocking issue"}},
			{result: &executor.RouterDecision{Route: "approve", Reason: "No blockers remain"}},
		},
	}

	if err := RunWith(cfg, 10, "", "", mock); err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}
	if mock.skillCallIdx != 4 {
		t.Errorf("expected 4 skill calls, got %d", mock.skillCallIdx)
	}
	if mock.routerCallIdx != 2 {
		t.Errorf("expected 2 router calls, got %d", mock.routerCallIdx)
	}
	if got := mock.routerInputs[0]; got != "needs more work" {
		t.Errorf("first router input = %q, want %q", got, "needs more work")
	}
}

func TestRunSkillExecutionError(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "review",
		Skills: map[string]config.Skill{
			"review": {
				Next: []config.Route{{ID: "approve", Done: true}},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: nil, err: fmt.Errorf("claude not found")},
		},
	}

	err := RunWith(cfg, 10, "", "", mock)
	if err == nil {
		t.Error("RunWith() should return error when executor fails")
	}
}

func TestRunRouterError(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "review",
		Router:            config.Agent{Runtime: "codex"},
		Skills: map[string]config.Skill{
			"review": {
				Next: []config.Route{
					{ID: "approve", Criteria: "done", Done: true},
					{ID: "rework", Criteria: "fix", Skill: "review"},
				},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "review output"}},
		},
		routerCalls: []mockRouterCall{
			{err: fmt.Errorf("router output invalid")},
		},
	}

	err := RunWith(cfg, 10, "", "", mock)
	if err == nil {
		t.Error("RunWith() should return error when router fails")
	}
}

func TestRunMaxIterationsReached(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "impl",
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{ID: "again", Skill: "impl"}},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "pass 1"}},
			{result: &executor.SkillResult{Stdout: "pass 2"}},
			{result: &executor.SkillResult{Stdout: "pass 3"}},
		},
	}

	err := RunWith(cfg, 3, "", "", mock)
	if err == nil {
		t.Error("RunWith() should return error when max iterations reached")
	}
}

func TestRunDefaultMaxIterations(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "review",
		Skills: map[string]config.Skill{
			"review": {
				Next: []config.Route{{ID: "approve", Done: true}},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "Done"}},
		},
	}

	if err := RunWith(cfg, 0, "", "", mock); err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}
}

func TestRunWithEntrypointOverride(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "impl",
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{ID: "done", Done: true}},
			},
			"review": {
				Next: []config.Route{{ID: "done", Done: true}},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "review done"}},
		},
	}

	if err := RunWith(cfg, 10, "", "review", mock); err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}
	if mock.skillCallIdx != 1 {
		t.Errorf("expected 1 call, got %d", mock.skillCallIdx)
	}
}

func TestRunWithEntrypointOverrideUnknownSkill(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "impl",
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{ID: "done", Done: true}},
			},
		},
	}

	mock := &mockExecutor{}

	err := RunWith(cfg, 10, "", "missing", mock)
	if err == nil {
		t.Error("RunWith() should return error when overridden entrypoint is unknown")
	}
}

func TestRunWithObserver(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "impl",
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{{ID: "review", Skill: "review"}},
			},
			"review": {
				Next: []config.Route{{ID: "approve", Done: true}},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "implemented"}},
			{result: &executor.SkillResult{Stdout: "reviewed"}},
		},
	}
	observer := &mockObserver{}

	if err := RunWithObserver(cfg, 10, "", "", mock, observer); err != nil {
		t.Fatalf("RunWithObserver() error: %v", err)
	}
	if got := len(observer.iterations); got != 2 {
		t.Fatalf("observer iterations = %d, want 2", got)
	}
	if observer.iterations[0] != 1 || observer.iterations[1] != 2 {
		t.Errorf("observer iterations = %v, want [1 2]", observer.iterations)
	}
	if observer.skills[0] != "impl" || observer.skills[1] != "review" {
		t.Errorf("observer skills = %v, want [impl review]", observer.skills)
	}
}

func TestRunPassesPreviousSkillStdoutToNextSkill(t *testing.T) {
	cfg := &config.Config{
		DefaultEntrypoint: "impl",
		Router:            config.Agent{Runtime: "codex"},
		Skills: map[string]config.Skill{
			"impl": {
				Next: []config.Route{
					{ID: "review", Criteria: "ready", Skill: "review"},
					{ID: "keep", Criteria: "continue", Skill: "impl"},
				},
			},
			"review": {
				Next: []config.Route{{ID: "approve", Done: true}},
			},
		},
	}

	mock := &mockExecutor{
		skillCalls: []mockSkillCall{
			{result: &executor.SkillResult{Stdout: "Implemented endpoint\nAdded tests"}},
			{result: &executor.SkillResult{Stdout: "Looks good"}},
		},
		routerCalls: []mockRouterCall{
			{result: &executor.RouterDecision{Route: "review", Reason: "Ready for review"}},
		},
	}

	if err := RunWith(cfg, 10, "Initial request", "", mock); err != nil {
		t.Fatalf("RunWith() error: %v", err)
	}

	if got := mock.skillInputs[0]; got != "Initial request" {
		t.Fatalf("first skill input = %q, want %q", got, "Initial request")
	}
	secondInput := mock.skillInputs[1]
	if !strings.Contains(secondInput, "Previous skill: impl") {
		t.Fatalf("second skill input missing previous skill: %q", secondInput)
	}
	if !strings.Contains(secondInput, "Selected route: review") {
		t.Fatalf("second skill input missing route id: %q", secondInput)
	}
	if !strings.Contains(secondInput, "Routing reason: Ready for review") {
		t.Fatalf("second skill input missing route reason: %q", secondInput)
	}
	if !strings.Contains(secondInput, "Implemented endpoint\nAdded tests") {
		t.Fatalf("second skill input missing previous stdout: %q", secondInput)
	}
}

func TestBuildHandoffTruncatesStdout(t *testing.T) {
	stdout := strings.Repeat("x", 13000)
	handoff := buildHandoff("impl", "review", "Ready for review", stdout)
	if !strings.Contains(handoff, "[truncated to last") {
		t.Fatalf("buildHandoff() should truncate long stdout: %q", handoff)
	}
}

func TestSelectRouteSingleRouteSkipsRouter(t *testing.T) {
	exec := &mockExecutor{}
	route, reason, err := selectRoute(exec, config.Agent{}, "hello", []config.Route{{ID: "finish", Done: true}}, "hello", executor.ExecutionOptions{})
	if err != nil {
		t.Fatalf("selectRoute() error: %v", err)
	}
	if route.ID != "finish" {
		t.Fatalf("route.ID = %q, want %q", route.ID, "finish")
	}
	if reason != "single available route" {
		t.Fatalf("reason = %q, want %q", reason, "single available route")
	}
	if exec.routerCallIdx != 0 {
		t.Fatalf("router should not be called, got %d calls", exec.routerCallIdx)
	}
}
