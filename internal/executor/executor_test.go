package executor

import (
	"reflect"
	"testing"
)

func TestExtractResultLastLine(t *testing.T) {
	text := `Here is my analysis of the code.
I found several issues.
{"summary": "Found issues <REVIEW_NG>"}`

	result, err := extractResult(text)
	if err != nil {
		t.Fatalf("extractResult() error: %v", err)
	}

	if result.Summary != "Found issues <REVIEW_NG>" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Found issues <REVIEW_NG>")
	}
}

func TestExtractResultMiddleLine(t *testing.T) {
	text := `Some preamble.
{"summary": "All fixed <REVIEW_OK>"}
Some trailing text.`

	result, err := extractResult(text)
	if err != nil {
		t.Fatalf("extractResult() error: %v", err)
	}

	if result.Summary != "All fixed <REVIEW_OK>" {
		t.Errorf("Summary = %q, want %q", result.Summary, "All fixed <REVIEW_OK>")
	}
}

func TestExtractResultWithCodeFence(t *testing.T) {
	text := "Some text\n```json\n{\"summary\": \"Completed <REVIEW_OK>\"}\n```"

	result, err := extractResult(text)
	if err != nil {
		t.Fatalf("extractResult() error: %v", err)
	}

	if result.Summary != "Completed <REVIEW_OK>" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Completed <REVIEW_OK>")
	}
}

func TestExtractResultNoJSON(t *testing.T) {
	text := "This output has no JSON at all."

	_, err := extractResult(text)
	if err == nil {
		t.Error("extractResult() should return error when no JSON found")
	}
}

func TestExtractResultEmptyInput(t *testing.T) {
	_, err := extractResult("")
	if err == nil {
		t.Error("extractResult() should return error for empty input")
	}
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name       string
		agent      string
		model      string
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
			name:       "codex",
			agent:      "codex",
			model:      "gpt-5",
			wantBinary: "codex",
			wantArgs:   []string{"exec", "prompt", "--model", "gpt-5"},
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
			gotBinary, gotArgs, err := buildCommand(tt.agent, tt.model, "prompt")
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

func TestParseOutput(t *testing.T) {
	output := []byte("some output\n{\"summary\": \"done <REVIEW_OK>\"}\n")

	res, err := parseOutput(output)
	if err != nil {
		t.Fatalf("parseOutput() error: %v", err)
	}

	if res.Summary != "done <REVIEW_OK>" {
		t.Fatalf("Summary = %q, want %q", res.Summary, "done <REVIEW_OK>")
	}
}
