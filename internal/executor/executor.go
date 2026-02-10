package executor

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
)

type SkillResult struct {
	Summary string
}

func ExecuteSkill(name string, agent string, model string, extraArgs []string, prevSummary string, routes []config.Route) (*SkillResult, error) {
	if agent == "" {
		agent = "claude"
	}

	fullPrompt := "/" + name + "\n"
	fullPrompt += "\nDo not output your reasoning in stdout.\n"
	if prevSummary != "" {
		fullPrompt += "\nPrevious skill output:\n" + prevSummary + "\n"
	}
	fullPrompt += buildRouteInstruction(routes)

	binary, args, err := buildCommand(agent, model, extraArgs, fullPrompt)
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

	return parseOutput(output)
}

func buildRouteInstruction(routes []config.Route) string {
	// Check if any route has routing criteria defined
	hasCriteria := false
	for _, r := range routes {
		if r.When != "" || r.Criteria != "" {
			hasCriteria = true
			break
		}
	}

	if !hasCriteria {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nStart your output with the appropriate status marker on the first line, then provide your detailed response:")

	for _, r := range routes {
		if r.When != "" {
			sb.WriteString(fmt.Sprintf("\n- %q: %s", r.When, r.Criteria))
		} else {
			sb.WriteString(fmt.Sprintf("\n- Otherwise: %s", r.Criteria))
		}
	}

	return sb.String()
}

func buildCommand(agent string, model string, extraArgs []string, prompt string) (string, []string, error) {
	switch agent {
	case "claude":
		args := []string{"-p", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, extraArgs...)
		return "claude", args, nil
	case "codex":
		args := []string{"exec", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, extraArgs...)
		return "codex", args, nil
	case "opencode":
		args := []string{"run", prompt}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, extraArgs...)
		return "opencode", args, nil
	default:
		return "", nil, fmt.Errorf("unsupported agent %q", agent)
	}
}

func parseOutput(output []byte) (*SkillResult, error) {
	return extractResult(string(output))
}

func extractResult(text string) (*SkillResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("no output from agent")
	}
	return &SkillResult{Summary: trimmed}, nil
}
