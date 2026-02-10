package executor

import (
	"reflect"
	"testing"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
)

func TestExtractResultFullOutput(t *testing.T) {
	text := "<REVIEW_NG>\nHere is my analysis of the code.\nI found several issues."

	result, err := extractResult(text)
	if err != nil {
		t.Fatalf("extractResult() error: %v", err)
	}

	want := "<REVIEW_NG>\nHere is my analysis of the code.\nI found several issues."
	if result.Summary != want {
		t.Errorf("Summary = %q, want %q", result.Summary, want)
	}
}

func TestExtractResultTrimsWhitespace(t *testing.T) {
	text := "  \n<REVIEW_OK>\nAll good.\n  \n"

	result, err := extractResult(text)
	if err != nil {
		t.Fatalf("extractResult() error: %v", err)
	}

	want := "<REVIEW_OK>\nAll good."
	if result.Summary != want {
		t.Errorf("Summary = %q, want %q", result.Summary, want)
	}
}

func TestExtractResultEmptyInput(t *testing.T) {
	_, err := extractResult("")
	if err == nil {
		t.Error("extractResult() should return error for empty input")
	}
}

func TestExtractResultWhitespaceOnly(t *testing.T) {
	_, err := extractResult("   \n  \n  ")
	if err == nil {
		t.Error("extractResult() should return error for whitespace-only input")
	}
}

func TestParseOutput(t *testing.T) {
	output := []byte("<REVIEW_OK>\nsome output\n")

	res, err := parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error: %v", err)
	}

	want := "<REVIEW_OK>\nsome output"
	if res.Summary != want {
		t.Fatalf("Summary = %q, want %q", res.Summary, want)
	}
}

func TestBuildRouteInstruction(t *testing.T) {
	tests := []struct {
		name   string
		routes []config.Route
		want   string
	}{
		{
			name:   "no routes",
			routes: nil,
			want:   "",
		},
		{
			name: "unconditional route without criteria",
			routes: []config.Route{
				{Skill: "2-review"},
			},
			want: "",
		},
		{
			name: "conditional routes with criteria",
			routes: []config.Route{
				{When: "<CONTINUE>", Criteria: "まだ作業が必要な場合", Skill: "1-impl"},
				{When: "<IMPL_DONE>", Criteria: "テストが全部通って実装が完了した場合", Skill: "2-review"},
			},
			want: "\nStart your output with the appropriate status marker on the first line, then provide your detailed response:" +
				"\n- \"<CONTINUE>\": まだ作業が必要な場合" +
				"\n- \"<IMPL_DONE>\": テストが全部通って実装が完了した場合",
		},
		{
			name: "conditional with default fallback",
			routes: []config.Route{
				{When: "<REVIEW_OK>", Criteria: "品質基準を満たしている場合", Skill: "<DONE>"},
				{Criteria: "改善が必要な場合", Skill: "1-impl"},
			},
			want: "\nStart your output with the appropriate status marker on the first line, then provide your detailed response:" +
				"\n- \"<REVIEW_OK>\": 品質基準を満たしている場合" +
				"\n- Otherwise: 改善が必要な場合",
		},
		{
			name: "multiple conditional routes",
			routes: []config.Route{
				{When: "<REVIEW_OK>", Criteria: "品質OK", Skill: "deploy"},
				{When: "<NEEDS_TEST>", Criteria: "テスト不足", Skill: "test"},
				{Criteria: "その他の改善が必要", Skill: "1-impl"},
			},
			want: "\nStart your output with the appropriate status marker on the first line, then provide your detailed response:" +
				"\n- \"<REVIEW_OK>\": 品質OK" +
				"\n- \"<NEEDS_TEST>\": テスト不足" +
				"\n- Otherwise: その他の改善が必要",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRouteInstruction(tt.routes)
			if got != tt.want {
				t.Errorf("buildRouteInstruction() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
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
