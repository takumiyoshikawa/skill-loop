package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/executor"
)

const DefaultMaxIterations = 100

// SkillExecutor abstracts skill execution for testability.
type SkillExecutor interface {
	ExecuteSkill(name string, agent string, model string, extraArgs []string, prevSummary string, routes []config.Route, opts executor.ExecutionOptions) (*executor.SkillResult, error)
}

type RunObserver interface {
	IterationStarted(iteration int, maxIterations int, skill string)
}

// defaultExecutor delegates to the real executor package.
type defaultExecutor struct{}

func (d *defaultExecutor) ExecuteSkill(name string, agent string, model string, extraArgs []string, prevSummary string, routes []config.Route, opts executor.ExecutionOptions) (*executor.SkillResult, error) {
	return executor.ExecuteSkill(name, agent, model, extraArgs, prevSummary, routes, opts)
}

func Run(cfg *config.Config, maxIterations int, prompt string, entrypoint string) error {
	return runWith(cfg, maxIterations, prompt, entrypoint, &defaultExecutor{}, nil)
}

func RunWith(cfg *config.Config, maxIterations int, prompt string, entrypoint string, exec SkillExecutor) error {
	return runWith(cfg, maxIterations, prompt, entrypoint, exec, nil)
}

func RunObserved(cfg *config.Config, maxIterations int, prompt string, entrypoint string, observer RunObserver) error {
	return runWith(cfg, maxIterations, prompt, entrypoint, &defaultExecutor{}, observer)
}

func RunWithObserver(cfg *config.Config, maxIterations int, prompt string, entrypoint string, exec SkillExecutor, observer RunObserver) error {
	return runWith(cfg, maxIterations, prompt, entrypoint, exec, observer)
}

func runWith(cfg *config.Config, maxIterations int, prompt string, entrypoint string, exec SkillExecutor, observer RunObserver) error {
	if maxIterations <= 0 {
		maxIterations = cfg.MaxIterations
	}
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	if entrypoint == "" {
		entrypoint = cfg.DefaultEntrypoint
	}

	if err := cfg.ValidateEntrypoint(entrypoint); err != nil {
		return err
	}

	currentSkill := entrypoint
	prevSummary := prompt

	for i := 0; i < maxIterations; i++ {
		skill, ok := cfg.Skills[currentSkill]
		if !ok {
			return fmt.Errorf("skill %q not found in config", currentSkill)
		}

		if observer != nil {
			observer.IterationStarted(i+1, maxIterations, currentSkill)
		}

		fmt.Printf("==> Running skill: %s (iteration %d)\n", currentSkill, i+1)

		runtime := skill.Agent.Runtime
		if runtime == "" {
			runtime = "claude"
		}

		result, err := exec.ExecuteSkill(
			currentSkill,
			runtime,
			skill.Agent.Model,
			skill.Agent.Args,
			prevSummary,
			skill.Next,
			executor.ExecutionOptions{
				IdleTimeout: time.Duration(cfg.IdleTimeoutSeconds) * time.Second,
				MaxRestarts: cfg.EffectiveMaxRestarts(),
			},
		)
		if err != nil {
			return fmt.Errorf("skill %q failed: %w", currentSkill, err)
		}

		fmt.Println(result.Summary)

		nextSkill := resolveNext(skill.Next, result.Summary)

		if nextSkill == "<DONE>" {
			fmt.Println("==> Loop finished.")
			return nil
		}

		if _, ok := cfg.Skills[nextSkill]; !ok {
			return fmt.Errorf("next skill %q (resolved from %q) not found in config", nextSkill, currentSkill)
		}

		prevSummary = result.Summary
		currentSkill = nextSkill
	}

	return fmt.Errorf("max iterations (%d) reached", maxIterations)
}

func resolveNext(routes []config.Route, summary string) string {
	for _, r := range routes {
		if r.When == "" || strings.Contains(summary, r.When) {
			return r.Skill
		}
	}
	return "<DONE>"
}
