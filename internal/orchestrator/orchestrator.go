package orchestrator

import (
	"fmt"
	"strings"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/executor"
)

const DefaultMaxIterations = 100

// SkillExecutor abstracts skill execution for testability.
type SkillExecutor interface {
	ExecuteSkill(name string, agent string, model string, prevSummary string) (*executor.SkillResult, error)
}

// defaultExecutor delegates to the real executor package.
type defaultExecutor struct{}

func (d *defaultExecutor) ExecuteSkill(name string, agent string, model string, prevSummary string) (*executor.SkillResult, error) {
	return executor.ExecuteSkill(name, agent, model, prevSummary)
}

func Run(cfg *config.Config, maxIterations int, prompt string, entrypoint string) error {
	return RunWith(cfg, maxIterations, prompt, entrypoint, &defaultExecutor{})
}

func RunWith(cfg *config.Config, maxIterations int, prompt string, entrypoint string, exec SkillExecutor) error {
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

		fmt.Printf("==> Running skill: %s (iteration %d)\n", currentSkill, i+1)

		runtime := skill.Agent.Runtime
		if runtime == "" {
			runtime = "claude"
		}

		result, err := exec.ExecuteSkill(currentSkill, runtime, skill.Agent.Model, prevSummary)
		if err != nil {
			return fmt.Errorf("skill %q failed: %w", currentSkill, err)
		}

		fmt.Printf("    Summary: %s\n", result.Summary)

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
