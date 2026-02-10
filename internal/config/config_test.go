package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: impl
skills:
  impl:
    agent:
      runtime: claude
      model: sonnet
    next:
      - skill: review
  review:
    next:
      - when: "<REVIEW_OK>"
        skill: "<DONE>"
      - skill: impl
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Entrypoint != "impl" {
		t.Errorf("Entrypoint = %q, want %q", cfg.Entrypoint, "impl")
	}

	if len(cfg.Skills) != 2 {
		t.Errorf("len(Skills) = %d, want 2", len(cfg.Skills))
	}

	if cfg.Skills["impl"].Agent.Model != "sonnet" {
		t.Errorf("Skills[impl].Agent.Model = %q, want %q", cfg.Skills["impl"].Agent.Model, "sonnet")
	}

	if cfg.Skills["review"].Agent.Model != "" {
		t.Errorf("Skills[review].Agent.Model = %q, want empty", cfg.Skills["review"].Agent.Model)
	}

	implRoutes := cfg.Skills["impl"].Next
	if len(implRoutes) != 1 {
		t.Fatalf("len(impl.Next) = %d, want 1", len(implRoutes))
	}
	if implRoutes[0].Skill != "review" {
		t.Errorf("impl.Next[0].Skill = %q, want %q", implRoutes[0].Skill, "review")
	}

	reviewRoutes := cfg.Skills["review"].Next
	if len(reviewRoutes) != 2 {
		t.Fatalf("len(review.Next) = %d, want 2", len(reviewRoutes))
	}
	if reviewRoutes[0].When != "<REVIEW_OK>" || reviewRoutes[0].Skill != "<DONE>" {
		t.Errorf("review.Next[0] = {%q, %q}, want {%q, %q}", reviewRoutes[0].When, reviewRoutes[0].Skill, "<REVIEW_OK>", "<DONE>")
	}
	if reviewRoutes[1].When != "" || reviewRoutes[1].Skill != "impl" {
		t.Errorf("review.Next[1] = {%q, %q}, want {%q, %q}", reviewRoutes[1].When, reviewRoutes[1].Skill, "", "impl")
	}
}

func TestLoadMissingEntrypoint(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`skills:
  review:
    next:
      - skill: "<DONE>"
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error for missing entrypoint")
	}
}

func TestLoadNoSkills(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: review
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error when no skills defined")
	}
}

func TestLoadEntrypointNotInSkills(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: missing
skills:
  review:
    next:
      - skill: "<DONE>"
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error when entrypoint not found in skills")
	}
}

func TestLoadRouteReferencesUnknownSkill(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: impl
skills:
  impl:
    next:
      - skill: nonexistent
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error when route references unknown skill")
	}
}

func TestLoadRouteEmptySkillTarget(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: impl
skills:
  impl:
    next:
      - when: "<OK>"
        skill: ""
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error when route has empty skill target")
	}
}

func TestLoadDoneIsValidTarget(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: impl
skills:
  impl:
    next:
      - skill: "<DONE>"
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Skills["impl"].Next[0].Skill != "<DONE>" {
		t.Errorf("expected <DONE> target, got %q", cfg.Skills["impl"].Next[0].Skill)
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte("this is not valid: yaml: [")
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error for invalid YAML")
	}
}

func TestLoadInvalidAgent(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: impl
skills:
  impl:
    agent:
      runtime: unknown
    next:
      - skill: "<DONE>"
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error for unsupported agent")
	}
}

func TestLoadLegacyAgentString(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`entrypoint: impl
skills:
  impl:
    agent: codex
    next:
      - skill: "<DONE>"
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error for legacy string agent format")
	}
}
