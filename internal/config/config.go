package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

type Route struct {
	ID         string `yaml:"id" jsonschema:"required,description=Stable route identifier returned by the router agent."`
	Criteria   string `yaml:"criteria,omitempty" jsonschema:"description=Judgment criteria used by the router agent when deciding whether this route should be chosen."`
	Skill      string `yaml:"skill,omitempty" jsonschema:"description=Target skill name to route to. Mutually exclusive with done."`
	Done       bool   `yaml:"done,omitempty" jsonschema:"description=Terminate the workflow when this route is selected. Mutually exclusive with skill."`
	LegacyWhen string `yaml:"when,omitempty" json:"-" jsonschema:"-"`
}

type Agent struct {
	Runtime string   `yaml:"runtime,omitempty" jsonschema:"description=Coding agent CLI runtime to execute. Supported values are claude or codex or opencode. Defaults to claude." default:"claude"`
	Model   string   `yaml:"model,omitempty" jsonschema:"description=Model ID to use for this skill (agent-specific)."`
	Args    []string `yaml:"args,omitempty" jsonschema:"description=Additional CLI arguments to pass to the agent (e.g. --dangerously-skip-permissions for claude)."`
}

type Skill struct {
	Agent Agent   `yaml:"agent,omitempty" jsonschema:"description=Agent configuration for this skill. runtime defaults to claude when omitted."`
	Next  []Route `yaml:"next" jsonschema:"required,description=Route options available after this skill runs. If more than one route exists then the shared router agent selects one by id."`
}

type Config struct {
	Name               string           `yaml:"name,omitempty" jsonschema:"description=Workflow name used for grouping run sessions under ~/.local/share/skill-loop/<name>/. When omitted the config filename is used."`
	Schedule           string           `yaml:"schedule,omitempty" jsonschema:"description=Optional cron schedule in standard 5-field crontab syntax. When set skill-loop stays resident and runs the workflow on each matching time."`
	Router             Agent            `yaml:"router,omitempty" jsonschema:"description=Shared router agent configuration used to choose the next route when a skill has multiple next routes."`
	DefaultEntrypoint  string           `yaml:"default_entrypoint" jsonschema:"required,description=Default skill to start the loop with. Must exist in the skills map."`
	MaxIterations      int              `yaml:"max_iterations,omitempty" jsonschema:"description=Maximum number of loop iterations before stopping. Defaults to 100 if omitted." default:"100"`
	IdleTimeoutSeconds int              `yaml:"idle_timeout_seconds,omitempty" jsonschema:"description=Idle timeout in seconds for each skill execution before restart. Defaults to 900 (15 minutes)." default:"900"`
	MaxRestarts        *int             `yaml:"max_restarts,omitempty" jsonschema:"description=Maximum automatic restarts per skill execution when idle timeout is exceeded. Defaults to 2. Set 0 to disable automatic restarts." default:"2"`
	Skills             map[string]Skill `yaml:"skills" jsonschema:"required,description=Map of skill names to their definitions."`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.DefaultEntrypoint == "" {
		return nil, fmt.Errorf("default_entrypoint is required")
	}

	if cfg.Schedule != "" {
		if _, err := cron.ParseStandard(cfg.Schedule); err != nil {
			return nil, fmt.Errorf("invalid schedule %q: %w", cfg.Schedule, err)
		}
	}

	if len(cfg.Skills) == 0 {
		return nil, fmt.Errorf("at least one skill is required")
	}

	if cfg.IdleTimeoutSeconds <= 0 {
		cfg.IdleTimeoutSeconds = 900
	}

	if cfg.MaxRestarts == nil {
		defaultMaxRestarts := 2
		cfg.MaxRestarts = &defaultMaxRestarts
	} else if *cfg.MaxRestarts < 0 {
		return nil, fmt.Errorf("max_restarts must be >= 0")
	}

	if err := cfg.ValidateEntrypoint(cfg.DefaultEntrypoint); err != nil {
		return nil, err
	}

	needsRouter := false
	for name, skill := range cfg.Skills {
		if err := validateAgent("skill "+strconvQuote(name), skill.Agent); err != nil {
			return nil, err
		}

		if len(skill.Next) == 0 {
			return nil, fmt.Errorf("skill %q: at least one next route is required", name)
		}
		if len(skill.Next) > 1 {
			needsRouter = true
		}

		seenRouteIDs := make(map[string]struct{}, len(skill.Next))
		for i, route := range skill.Next {
			if route.LegacyWhen != "" {
				return nil, fmt.Errorf("skill %q: route[%d] uses deprecated when matcher; use id + criteria + router instead", name, i)
			}
			if strings.TrimSpace(route.ID) == "" {
				return nil, fmt.Errorf("skill %q: route[%d] requires id", name, i)
			}
			if _, ok := seenRouteIDs[route.ID]; ok {
				return nil, fmt.Errorf("skill %q: route[%d] reuses id %q", name, i, route.ID)
			}
			seenRouteIDs[route.ID] = struct{}{}
			if route.Skill == "<DONE>" {
				return nil, fmt.Errorf("skill %q: route[%d] uses deprecated <DONE>; use done: true instead", name, i)
			}
			if len(skill.Next) > 1 && strings.TrimSpace(route.Criteria) == "" {
				return nil, fmt.Errorf("skill %q: route[%d] requires criteria when multiple routes are present", name, i)
			}
			if route.Done {
				if route.Skill != "" {
					return nil, fmt.Errorf("skill %q: route[%d] cannot set both done and skill", name, i)
				}
				continue
			}
			if route.Skill == "" {
				return nil, fmt.Errorf("skill %q: route[%d] must set either skill or done", name, i)
			}
			if _, ok := cfg.Skills[route.Skill]; !ok {
				return nil, fmt.Errorf("skill %q: route[%d] references unknown skill %q", name, i, route.Skill)
			}
		}
	}

	if needsRouter {
		if isZeroAgent(cfg.Router) {
			return nil, fmt.Errorf("router is required when any skill defines multiple next routes")
		}
	}
	if !isZeroAgent(cfg.Router) {
		if err := validateAgent("router", cfg.Router); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

func (c *Config) EffectiveMaxRestarts() int {
	if c.MaxRestarts == nil {
		return 2
	}
	return *c.MaxRestarts
}

func (c *Config) EffectiveName(path string) string {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		base := filepath.Base(path)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return sanitizeName(name)
}

func (c *Config) ValidateEntrypoint(entrypoint string) error {
	if entrypoint == "" {
		return fmt.Errorf("entrypoint is required")
	}

	if _, ok := c.Skills[entrypoint]; !ok {
		return fmt.Errorf("entrypoint %q not found in skills", entrypoint)
	}

	return nil
}

func sanitizeName(input string) string {
	var b strings.Builder
	lastDash := false

	for _, r := range strings.TrimSpace(input) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
		case r == '-', r == '_':
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		default:
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		}
	}

	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "default"
	}
	return name
}

func validateAgent(label string, agent Agent) error {
	runtime := agent.Runtime
	if runtime == "" {
		runtime = "claude"
	}
	if runtime != "claude" && runtime != "codex" && runtime != "opencode" {
		return fmt.Errorf("%s: unsupported agent runtime %q (supported: claude, codex, opencode)", label, agent.Runtime)
	}
	return nil
}

func isZeroAgent(agent Agent) bool {
	return agent.Runtime == "" && agent.Model == "" && len(agent.Args) == 0
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}
