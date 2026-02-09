package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Route struct {
	When  string `yaml:"when,omitempty" jsonschema:"description=Substring to match in the previous skill's summary. If omitted or empty the route always matches (default route)."`
	Skill string `yaml:"skill" jsonschema:"required,description=Target skill name to route to or <DONE> to terminate the loop."`
}

type Skill struct {
	Model string  `yaml:"model,omitempty" jsonschema:"description=Claude model ID to use for this skill (e.g. claude-sonnet-4-5-20250929)."`
	Next  []Route `yaml:"next" jsonschema:"required,description=Routing rules evaluated top-to-bottom. The first matching route is used."`
}

type Config struct {
	Entrypoint    string           `yaml:"entrypoint" jsonschema:"required,description=Name of the skill to start the loop with. Must exist in the skills map."`
	MaxIterations int              `yaml:"max_iterations,omitempty" jsonschema:"description=Maximum number of loop iterations before stopping. Defaults to 100 if omitted." default:"100"`
	Skills        map[string]Skill `yaml:"skills" jsonschema:"required,description=Map of skill names to their definitions."`
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

	if cfg.Entrypoint == "" {
		return nil, fmt.Errorf("entrypoint is required")
	}

	if len(cfg.Skills) == 0 {
		return nil, fmt.Errorf("at least one skill is required")
	}

	if _, ok := cfg.Skills[cfg.Entrypoint]; !ok {
		return nil, fmt.Errorf("entrypoint %q not found in skills", cfg.Entrypoint)
	}

	for name, skill := range cfg.Skills {
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
