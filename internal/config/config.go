package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Route struct {
	When     string `yaml:"when,omitempty" jsonschema:"description=Substring to match in the previous skill's summary. If omitted or empty the route always matches (default route)."`
	Criteria string `yaml:"criteria,omitempty" jsonschema:"description=Judgment criteria for when this route should be chosen. Included in the agent prompt to guide decision-making."`
	Skill    string `yaml:"skill" jsonschema:"required,description=Target skill name to route to or <DONE> to terminate the loop."`
}

type Agent struct {
	Runtime string   `yaml:"runtime,omitempty" jsonschema:"description=Coding agent CLI runtime to execute. Supported values are claude or codex or opencode. Defaults to claude." default:"claude"`
	Model   string   `yaml:"model,omitempty" jsonschema:"description=Model ID to use for this skill (agent-specific)."`
	Args    []string `yaml:"args,omitempty" jsonschema:"description=Additional CLI arguments to pass to the agent (e.g. --dangerously-skip-permissions for claude)."`
}

type Skill struct {
	Agent Agent   `yaml:"agent,omitempty" jsonschema:"description=Agent configuration for this skill. runtime defaults to claude when omitted."`
	Next  []Route `yaml:"next" jsonschema:"required,description=Routing rules evaluated top-to-bottom. The first matching route is used."`
}

type Config struct {
	DefaultEntrypoint string           `yaml:"default_entrypoint" jsonschema:"required,description=Default skill to start the loop with. Must exist in the skills map."`
	MaxIterations     int              `yaml:"max_iterations,omitempty" jsonschema:"description=Maximum number of loop iterations before stopping. Defaults to 100 if omitted." default:"100"`
	Skills            map[string]Skill `yaml:"skills" jsonschema:"required,description=Map of skill names to their definitions."`
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

	if len(cfg.Skills) == 0 {
		return nil, fmt.Errorf("at least one skill is required")
	}

	if err := cfg.ValidateEntrypoint(cfg.DefaultEntrypoint); err != nil {
		return nil, err
	}

	for name, skill := range cfg.Skills {
		runtime := skill.Agent.Runtime
		if runtime == "" {
			runtime = "claude"
		}
		if runtime != "claude" && runtime != "codex" && runtime != "opencode" {
			return nil, fmt.Errorf("skill %q: unsupported agent runtime %q (supported: claude, codex, opencode)", name, skill.Agent.Runtime)
		}

		for i, route := range skill.Next {
			if route.Skill == "" {
				return nil, fmt.Errorf("skill %q: route[%d] has empty skill target", name, i)
			}
			if route.Skill != "<DONE>" {
				if _, ok := cfg.Skills[route.Skill]; !ok {
					return nil, fmt.Errorf("skill %q: route[%d] references unknown skill %q", name, i, route.Skill)
				}
			}
		}
	}

	return &cfg, nil
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
