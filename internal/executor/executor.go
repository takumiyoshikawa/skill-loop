package executor

import (
	"bytes"
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
	defaultIdleTimeout = 900 * time.Second
	defaultMaxRestarts = 2
)

type SkillResult struct {
	Summary string
}

type ExecutionOptions struct {
	IdleTimeout time.Duration
	MaxRestarts int
}

func ExecuteSkill(name string, agent string, model string, extraArgs []string, prevSummary string, routes []config.Route, opts ExecutionOptions) (*SkillResult, error) {
	if agent == "" {
		agent = "claude"
	}

	opts = normalizeOptions(opts)

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

	attempt := 0
	for {
		result, err := executeSkillOnce(agent, binary, args, opts.IdleTimeout)
		if err == nil {
			return result, nil
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

func buildRouteInstruction(routes []config.Route) string {
	// Check if any route has routing criteria defined (when or criteria field)
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
			if r.Criteria != "" {
				sb.WriteString(fmt.Sprintf("\n- %q: %s", r.When, r.Criteria))
			} else {
				sb.WriteString(fmt.Sprintf("\n- %q", r.When))
			}
		} else {
			if r.Criteria != "" {
				sb.WriteString(fmt.Sprintf("\n- Otherwise: %s", r.Criteria))
			}
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

func executeSkillOnce(agent string, binary string, args []string, idleTimeout time.Duration) (*SkillResult, error) {
	cmd := exec.Command(binary, args...)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	lastActivity := &activityClock{last: time.Now()}
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf, lastActivity)
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
			return parseOutput(stdoutBuf.Bytes())
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

func tailString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[len(s)-maxBytes:])
}
