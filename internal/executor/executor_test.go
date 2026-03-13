package executor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
)

func TestParseSkillOutputFullOutput(t *testing.T) {
	result, err := parseSkillOutput([]byte("Implemented feature.\nAdded tests.\n"))
	if err != nil {
		t.Fatalf("parseSkillOutput() error: %v", err)
	}

	want := "Implemented feature.\nAdded tests."
	if result.Stdout != want {
		t.Errorf("Stdout = %q, want %q", result.Stdout, want)
	}
}

func TestParseSkillOutputEmptyInput(t *testing.T) {
	_, err := parseSkillOutput([]byte(" \n\t "))
	if err == nil {
		t.Error("parseSkillOutput() should return error for whitespace-only input")
	}
}

func TestParseRouterOutput(t *testing.T) {
	routes := []config.Route{
		{ID: "approve", Done: true},
		{ID: "rework", Skill: "impl"},
	}

	got, err := parseRouterOutput([]byte(`{"route":"approve","reason":"Looks good"}`), routes)
	if err != nil {
		t.Fatalf("parseRouterOutput() error: %v", err)
	}

	want := &RouterDecision{Route: "approve", Reason: "Looks good"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decision = %#v, want %#v", got, want)
	}
}

func TestParseRouterOutputRejectsUnknownRoute(t *testing.T) {
	_, err := parseRouterOutput([]byte(`{"route":"missing","reason":"oops"}`), []config.Route{{ID: "approve", Done: true}})
	if err == nil {
		t.Fatal("parseRouterOutput() expected error for unknown route")
	}
}

func TestParseRouterOutputRejectsMissingReason(t *testing.T) {
	_, err := parseRouterOutput([]byte(`{"route":"approve","reason":"   "}`), []config.Route{{ID: "approve", Done: true}})
	if err == nil {
		t.Fatal("parseRouterOutput() expected error for empty reason")
	}
}

func TestBuildSkillPrompt(t *testing.T) {
	got := buildSkillPrompt("review", "Previous skill: impl\nRouting reason: tests failed")
	if !strings.Contains(got, "/review") {
		t.Fatalf("buildSkillPrompt() missing skill invocation: %q", got)
	}
	if !strings.Contains(got, "Context from skill-loop:") {
		t.Fatalf("buildSkillPrompt() missing context header: %q", got)
	}
	if !strings.Contains(got, "Routing reason: tests failed") {
		t.Fatalf("buildSkillPrompt() missing handoff details: %q", got)
	}
}

func TestBuildRouterPrompt(t *testing.T) {
	routes := []config.Route{
		{ID: "approve", Criteria: "ship it", Done: true},
		{ID: "rework", Criteria: "needs changes", Skill: "impl"},
	}

	got := buildRouterPrompt("review", "There is one blocking issue.", routes)
	if !strings.Contains(got, `JSON shape: {"route":"<route-id>","reason":"<short reason>"}`) {
		t.Fatalf("buildRouterPrompt() missing json contract: %q", got)
	}
	if !strings.Contains(got, "- approve: ship it (done)") {
		t.Fatalf("buildRouterPrompt() missing done route: %q", got)
	}
	if !strings.Contains(got, "- rework: needs changes (next skill: impl)") {
		t.Fatalf("buildRouterPrompt() missing skill route: %q", got)
	}
	if !strings.Contains(got, "Skill stdout:\nThere is one blocking issue.") {
		t.Fatalf("buildRouterPrompt() missing skill stdout: %q", got)
	}
}

func TestBuildRouterRepairPrompt(t *testing.T) {
	got := buildRouterRepairPrompt(
		"review",
		"needs more work",
		[]config.Route{{ID: "rework", Criteria: "needs changes", Skill: "impl"}},
		"approve",
		assertErr("not json"),
	)
	if !strings.Contains(got, "Your previous response was invalid.") {
		t.Fatalf("buildRouterRepairPrompt() missing repair note: %q", got)
	}
	if !strings.Contains(got, "Previous invalid response:\napprove") {
		t.Fatalf("buildRouterRepairPrompt() missing invalid response: %q", got)
	}
}

func TestFormatPromptTextTruncatesFromTail(t *testing.T) {
	input := strings.Repeat("a", promptTextCharLimit+20)
	got := FormatPromptText(input)
	if !strings.Contains(got, "[truncated to last") {
		t.Fatalf("FormatPromptText() should indicate truncation: %q", got)
	}
	if !strings.HasSuffix(got, strings.Repeat("a", promptTextCharLimit)) {
		t.Fatal("FormatPromptText() should keep the tail of the text")
	}
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name       string
		agent      string
		model      string
		extraArgs  []string
		wantBinary string
		wantArgs   []string
		wantErr    bool
	}{
		{
			name:       "claude",
			agent:      "claude",
			model:      "sonnet",
			wantBinary: "claude",
			wantArgs:   []string{"-p", "prompt", "--model", "sonnet"},
		},
		{
			name:       "claude with extra args",
			agent:      "claude",
			model:      "sonnet",
			extraArgs:  []string{"--dangerously-skip-permissions"},
			wantBinary: "claude",
			wantArgs:   []string{"-p", "prompt", "--model", "sonnet", "--dangerously-skip-permissions"},
		},
		{
			name:       "codex",
			agent:      "codex",
			model:      "gpt-5",
			wantBinary: "codex",
			wantArgs:   []string{"exec", "prompt", "--model", "gpt-5"},
		},
		{
			name:       "codex with extra args",
			agent:      "codex",
			model:      "gpt-5",
			extraArgs:  []string{"--full-auto"},
			wantBinary: "codex",
			wantArgs:   []string{"exec", "prompt", "--model", "gpt-5", "--full-auto"},
		},
		{
			name:       "opencode",
			agent:      "opencode",
			model:      "x",
			wantBinary: "opencode",
			wantArgs:   []string{"run", "prompt", "--model", "x"},
		},
		{
			name:    "invalid",
			agent:   "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBinary, gotArgs, err := buildCommand(tt.agent, tt.model, tt.extraArgs, "prompt")
			if tt.wantErr {
				if err == nil {
					t.Fatal("buildCommand() expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("buildCommand() error: %v", err)
			}
			if gotBinary != tt.wantBinary {
				t.Fatalf("binary = %q, want %q", gotBinary, tt.wantBinary)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("args = %#v, want %#v", gotArgs, tt.wantArgs)
			}
		})
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

func assertErr(message string) error {
	return staticErr(message)
}
