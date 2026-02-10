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

func ExecuteSkill(name string, agent string, model string, prevSummary string) (*SkillResult, error) {
	if agent == "" {
		agent = "claude"
	}

	fullPrompt := "Run the skill: " + name + "\n"
	if prevSummary != "" {
		fullPrompt += "\nPrevious skill summary: " + prevSummary + "\n"
	}
	fullPrompt += jsonInstruction

	binary, args, err := buildCommand(agent, model, fullPrompt)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binary, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s command failed: %w\nstderr: %s", agent, err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("%s command failed: %w", agent, err)
	}

	return parseOutput(agent, output)
}

func buildCommand(agent string, model string, prompt string) (string, []string, error) {
	switch agent {
	case "claude":
		args := []string{"-p", prompt, "--output-format", "json"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return "claude", args, nil
	case "codex":
		args := []string{"exec", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
		return "codex", args, nil
	case "opencode":
		args := []string{"run", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
		return "opencode", args, nil
	default:
		return "", nil, fmt.Errorf("unsupported agent %q", agent)
	}
}

func parseOutput(agent string, output []byte) (*SkillResult, error) {
	if agent == "claude" {
		return parseClaudeOutput(output)
	}

	return extractResult(string(output))
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
