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
	ExecuteSkill(name string, agent config.Agent, input string, opts executor.ExecutionOptions) (*executor.SkillResult, error)
	RouteSkillOutput(skillName string, router config.Agent, output string, routes []config.Route, opts executor.ExecutionOptions) (*executor.RouterDecision, error)
}

type RunObserver interface {
	IterationStarted(iteration int, maxIterations int, skill string)
}

type defaultExecutor struct{}

func (d *defaultExecutor) ExecuteSkill(name string, agent config.Agent, input string, opts executor.ExecutionOptions) (*executor.SkillResult, error) {
	return executor.ExecuteSkill(name, agent, input, opts)
}

func (d *defaultExecutor) RouteSkillOutput(skillName string, router config.Agent, output string, routes []config.Route, opts executor.ExecutionOptions) (*executor.RouterDecision, error) {
	return executor.RouteSkillOutput(skillName, router, output, routes, opts)
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
	handoff := strings.TrimSpace(prompt)

	for i := 0; i < maxIterations; i++ {
		skill, ok := cfg.Skills[currentSkill]
		if !ok {
			return fmt.Errorf("skill %q not found in config", currentSkill)
		}

		if observer != nil {
			observer.IterationStarted(i+1, maxIterations, currentSkill)
		}

		fmt.Printf("==> Running skill: %s (iteration %d)\n", currentSkill, i+1)

		opts := executor.ExecutionOptions{
			IdleTimeout: time.Duration(cfg.IdleTimeoutSeconds) * time.Second,
			MaxRestarts: cfg.EffectiveMaxRestarts(),
		}

		result, err := exec.ExecuteSkill(currentSkill, skill.Agent, handoff, opts)
		if err != nil {
			return fmt.Errorf("skill %q failed: %w", currentSkill, err)
		}

		route, reason, err := selectRoute(exec, cfg.Router, currentSkill, skill.Next, result.Stdout, opts)
		if err != nil {
			return fmt.Errorf("skill %q routing failed: %w", currentSkill, err)
		}

		fmt.Printf("==> Router selected: %s", route.ID)
		if reason != "" {
			fmt.Printf(" (%s)", reason)
		}
		fmt.Println()

		if route.Done {
			fmt.Println("==> Loop finished.")
			return nil
		}

		if _, ok := cfg.Skills[route.Skill]; !ok {
			return fmt.Errorf("next skill %q (resolved from route %q) not found in config", route.Skill, route.ID)
		}

		handoff = buildHandoff(currentSkill, route.ID, reason, result.Stdout)
		currentSkill = route.Skill
	}

	return fmt.Errorf("max iterations (%d) reached", maxIterations)
}

func selectRoute(exec SkillExecutor, router config.Agent, skillName string, routes []config.Route, output string, opts executor.ExecutionOptions) (config.Route, string, error) {
	if len(routes) == 0 {
		return config.Route{}, "", fmt.Errorf("no next routes configured")
	}
	if len(routes) == 1 {
		reason := routes[0].Criteria
		if strings.TrimSpace(reason) == "" {
			reason = "single available route"
		}
		return routes[0], reason, nil
	}

	decision, err := exec.RouteSkillOutput(skillName, router, output, routes, opts)
	if err != nil {
		return config.Route{}, "", err
	}

	for _, route := range routes {
		if route.ID == decision.Route {
			return route, decision.Reason, nil
		}
	}

	return config.Route{}, "", fmt.Errorf("unknown route %q", decision.Route)
}

func buildHandoff(previousSkill string, routeID string, reason string, stdout string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Previous skill: %s\n", previousSkill)
	fmt.Fprintf(&sb, "Selected route: %s\n", routeID)
	if strings.TrimSpace(reason) != "" {
		fmt.Fprintf(&sb, "Routing reason: %s\n", reason)
	}
	sb.WriteString("\nPrevious skill stdout:\n")
	sb.WriteString(executor.FormatPromptText(stdout))
	return sb.String()
}
