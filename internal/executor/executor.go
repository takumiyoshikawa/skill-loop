package executor

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const jsonInstruction = `

After completing the task, output ONLY the following JSON on the last line:
{"summary": "<brief summary of what you did>"}`

type SkillResult struct {
	Summary string `json:"summary"`
}

// claudeResponse represents the --output-format json output from Claude Code CLI.
type claudeResponse struct {
	Result string `json:"result"`
}

func ExecuteSkill(name string, model string, prevSummary string) (*SkillResult, error) {
	fullPrompt := "Run the skill: " + name + "\n"
	if prevSummary != "" {
		fullPrompt += "\nPrevious skill summary: " + prevSummary + "\n"
	}
	fullPrompt += jsonInstruction

	args := []string{"-p", fullPrompt, "--output-format", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.Command("claude", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("claude command failed: %w\nstderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("claude command failed: %w", err)
	}

	return parseClaudeOutput(output)
}

func parseClaudeOutput(output []byte) (*SkillResult, error) {
	var resp claudeResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse claude output: %w", err)
	}

	if resp.Result == "" {
		return nil, fmt.Errorf("empty result from claude")
	}

	return extractResult(resp.Result)
}

func extractResult(text string) (*SkillResult, error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")

	// Try lines from the end to find a valid JSON object
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		// Strip markdown code fences if present
		line = strings.TrimPrefix(line, "```json")
		line = strings.TrimPrefix(line, "```")
		line = strings.TrimSuffix(line, "```")
		line = strings.TrimSpace(line)

		if !strings.HasPrefix(line, "{") {
			continue
		}

		var result SkillResult
		if err := json.Unmarshal([]byte(line), &result); err == nil {
			return &result, nil
		}
	}

	return nil, fmt.Errorf("no valid JSON result found in assistant output")
}
