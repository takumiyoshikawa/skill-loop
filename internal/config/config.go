package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Route struct {
	When  string `yaml:"when,omitempty"`
	Skill string `yaml:"skill"`
}

type Skill struct {
	Model string  `yaml:"model,omitempty"`
	Next  []Route `yaml:"next"`
}

type Config struct {
	Entrypoint    string           `yaml:"entrypoint"`
	MaxIterations int              `yaml:"max_iterations,omitempty"`
	Skills        map[string]Skill `yaml:"skills"`
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
