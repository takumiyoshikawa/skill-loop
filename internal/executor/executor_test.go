package executor

import (
	"encoding/json"
	"testing"
)

func TestParseClaudeOutput(t *testing.T) {
	resp := claudeResponse{
		Result: `I reviewed the code and found some issues.
{"summary": "Found 3 bugs <REVIEW_NG>"}`,
	}
	data, _ := json.Marshal(resp)

	result, err := parseClaudeOutput(data)
	if err != nil {
		t.Fatalf("parseClaudeOutput() error: %v", err)
	}

	if result.Summary != "Found 3 bugs <REVIEW_NG>" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Found 3 bugs <REVIEW_NG>")
	}
}

func TestParseClaudeOutputEmptyResult(t *testing.T) {
	resp := claudeResponse{Result: ""}
	data, _ := json.Marshal(resp)

	_, err := parseClaudeOutput(data)
	if err == nil {
		t.Error("parseClaudeOutput() should return error for empty result")
	}
}

func TestParseClaudeOutputInvalidJSON(t *testing.T) {
	_, err := parseClaudeOutput([]byte("not json"))
	if err == nil {
		t.Error("parseClaudeOutput() should return error for invalid JSON")
	}
}

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
