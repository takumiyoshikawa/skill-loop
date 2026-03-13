package executor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
)

const (
	defaultIdleTimeout   = 900 * time.Second
	defaultMaxRestarts   = 2
	promptTextCharLimit  = 12000
	routerRepairAttempts = 1
)

type SkillResult struct {
	Stdout string
}

type RouterDecision struct {
	Route  string `json:"route"`
	Reason string `json:"reason"`
}

type ExecutionOptions struct {
	IdleTimeout time.Duration
	MaxRestarts int
}

func ExecuteSkill(name string, agent config.Agent, input string, opts ExecutionOptions) (*SkillResult, error) {
	opts = normalizeOptions(opts)

	prompt := buildSkillPrompt(name, input)
	binary, args, err := buildCommand(normalizeAgentRuntime(agent.Runtime), agent.Model, agent.Args, prompt)
	if err != nil {
		return nil, err
	}

	attempt := 0
	for {
		output, err := executeCommand(normalizeAgentRuntime(agent.Runtime), binary, args, opts.IdleTimeout, true)
		if err == nil {
			return parseSkillOutput(output)
		}

		var idleErr *idleTimeoutError
		if errors.As(err, &idleErr) && attempt < opts.MaxRestarts {
			attempt++
			fmt.Fprintf(os.Stderr, "[skill-loop] skill %s idle for %s, restarting (%d/%d)\n", name, opts.IdleTimeout, attempt, opts.MaxRestarts)
			continue
		}

		return nil, err
	}
}

func RouteSkillOutput(skillName string, router config.Agent, output string, routes []config.Route, opts ExecutionOptions) (*RouterDecision, error) {
	opts = normalizeOptions(opts)
	runtime := normalizeAgentRuntime(router.Runtime)
	prompt := buildRouterPrompt(skillName, output, routes)

	for repair := 0; repair <= routerRepairAttempts; repair++ {
		binary, args, err := buildCommand(runtime, router.Model, router.Args, prompt)
		if err != nil {
			return nil, err
		}

		raw, err := executeCommand(runtime, binary, args, opts.IdleTimeout, false)
		if err != nil {
			return nil, err
		}

		decision, err := parseRouterOutput(raw, routes)
		if err == nil {
			return decision, nil
		}
		if repair == routerRepairAttempts {
			return nil, fmt.Errorf("router output invalid: %w", err)
		}

		prompt = buildRouterRepairPrompt(skillName, output, routes, string(raw), err)
	}

	return nil, fmt.Errorf("router output invalid")
}

func FormatPromptText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "(empty)"
	}

	runes := []rune(trimmed)
	if len(runes) <= promptTextCharLimit {
		return trimmed
	}

	return fmt.Sprintf("[truncated to last %d of %d characters]\n%s", promptTextCharLimit, len(runes), string(runes[len(runes)-promptTextCharLimit:]))
}

func buildSkillPrompt(name string, input string) string {
	var sb strings.Builder
	sb.WriteString("/")
	sb.WriteString(name)
	sb.WriteString("\n\nDo not output your reasoning in stdout.\n")
	if strings.TrimSpace(input) != "" {
		sb.WriteString("\nContext from skill-loop:\n")
		sb.WriteString(FormatPromptText(input))
		sb.WriteString("\n")
	}
	return sb.String()
}

func buildRouterPrompt(skillName string, output string, routes []config.Route) string {
	var sb strings.Builder
	sb.WriteString("You are the router for skill-loop.\n")
	sb.WriteString("Choose exactly one route based on the skill output and the route criteria.\n")
	sb.WriteString("Respond with exactly one JSON object and nothing else.\n")
	sb.WriteString("Do not use markdown fences.\n")
	sb.WriteString(`JSON shape: {"route":"<route-id>","reason":"<short reason>"}` + "\n\n")
	fmt.Fprintf(&sb, "Current skill: %s\n\n", skillName)
	sb.WriteString("Available routes:\n")
	for _, route := range routes {
		fmt.Fprintf(&sb, "- %s: %s", route.ID, route.Criteria)
		if route.Done {
			sb.WriteString(" (done)")
		} else {
			fmt.Fprintf(&sb, " (next skill: %s)", route.Skill)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nSkill stdout:\n")
	sb.WriteString(FormatPromptText(output))
	sb.WriteString("\n")
	return sb.String()
}

func buildRouterRepairPrompt(skillName string, output string, routes []config.Route, invalidOutput string, parseErr error) string {
	var sb strings.Builder
	sb.WriteString(buildRouterPrompt(skillName, output, routes))
	sb.WriteString("\nYour previous response was invalid.\n")
	fmt.Fprintf(&sb, "Validation error: %s\n", parseErr)
	sb.WriteString("Previous invalid response:\n")
	sb.WriteString(FormatPromptText(invalidOutput))
	sb.WriteString("\nReturn only valid JSON now.\n")
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

func parseSkillOutput(output []byte) (*SkillResult, error) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, fmt.Errorf("no output from agent")
	}
	return &SkillResult{Stdout: trimmed}, nil
}

func parseRouterOutput(output []byte, routes []config.Route) (*RouterDecision, error) {
	var decision RouterDecision
	if err := json.Unmarshal(output, &decision); err != nil {
		return nil, fmt.Errorf("parse router json: %w", err)
	}
	decision.Route = strings.TrimSpace(decision.Route)
	decision.Reason = strings.TrimSpace(decision.Reason)
	if decision.Route == "" {
		return nil, fmt.Errorf("route is required")
	}
	if decision.Reason == "" {
		return nil, fmt.Errorf("reason is required")
	}
	for _, route := range routes {
		if route.ID == decision.Route {
			return &decision, nil
		}
	}
	return nil, fmt.Errorf("unknown route %q", decision.Route)
}

func executeCommand(agent string, binary string, args []string, idleTimeout time.Duration, streamStdout bool) ([]byte, error) {
	cmd := exec.Command(binary, args...)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	lastActivity := &activityClock{last: time.Now()}
	stdoutWriter := io.MultiWriter(&stdoutBuf, lastActivity)
	if streamStdout {
		stdoutWriter = io.MultiWriter(os.Stdout, &stdoutBuf, lastActivity)
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf, lastActivity)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s command failed: %w", agent, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					tail := tailString(stderrBuf.String(), 4000)
					if tail != "" {
						return nil, fmt.Errorf("%s command failed: %w\nstderr: %s", agent, exitErr, tail)
					}
				}
				return nil, fmt.Errorf("%s command failed: %w", agent, err)
			}
			return stdoutBuf.Bytes(), nil
		case <-ticker.C:
			if idleTimeout > 0 && time.Since(lastActivity.Get()) > idleTimeout {
				_ = cmd.Process.Kill()
				<-done
				return nil, &idleTimeoutError{Duration: idleTimeout}
			}
		}
	}
}

func normalizeOptions(opts ExecutionOptions) ExecutionOptions {
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = defaultIdleTimeout
	}
	if opts.MaxRestarts < 0 {
		opts.MaxRestarts = defaultMaxRestarts
	}
	return opts
}

func normalizeAgentRuntime(runtime string) string {
	if runtime == "" {
		return "claude"
	}
	return runtime
}

type idleTimeoutError struct {
	Duration time.Duration
}

func (e *idleTimeoutError) Error() string {
	return fmt.Sprintf("idle timeout exceeded (%s)", e.Duration)
}

type activityClock struct {
	mu   sync.Mutex
	last time.Time
}

func (a *activityClock) Write(p []byte) (int, error) {
	a.mu.Lock()
	a.last = time.Now()
	a.mu.Unlock()
	return len(p), nil
}

func (a *activityClock) Get() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

func tailString(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[len(runes)-max:])
}
