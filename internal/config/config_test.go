package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`default_entrypoint: impl
skills:
  impl:
    agent:
      runtime: claude
      model: sonnet
    next:
      - when: "<CONTINUE>"
        criteria: "まだ作業が必要な場合"
        skill: impl
      - when: "<IMPL_DONE>"
        criteria: "実装が完了した場合"
        skill: review
  review:
    next:
      - when: "<REVIEW_OK>"
        criteria: "品質基準を満たしている場合"
        skill: "<DONE>"
      - criteria: "改善が必要な場合"
        skill: impl
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.DefaultEntrypoint != "impl" {
		t.Errorf("DefaultEntrypoint = %q, want %q", cfg.DefaultEntrypoint, "impl")
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

	if cfg.IdleTimeoutSeconds != 900 {
		t.Errorf("IdleTimeoutSeconds = %d, want 900", cfg.IdleTimeoutSeconds)
	}

	if cfg.EffectiveMaxRestarts() != 2 {
		t.Errorf("EffectiveMaxRestarts = %d, want 2", cfg.EffectiveMaxRestarts())
	}

	implRoutes := cfg.Skills["impl"].Next
	if len(implRoutes) != 2 {
		t.Fatalf("len(impl.Next) = %d, want 2", len(implRoutes))
	}
	if implRoutes[0].When != "<CONTINUE>" || implRoutes[0].Criteria != "まだ作業が必要な場合" || implRoutes[0].Skill != "impl" {
		t.Errorf("impl.Next[0] = %+v", implRoutes[0])
	}
	if implRoutes[1].When != "<IMPL_DONE>" || implRoutes[1].Criteria != "実装が完了した場合" || implRoutes[1].Skill != "review" {
		t.Errorf("impl.Next[1] = %+v", implRoutes[1])
	}

	reviewRoutes := cfg.Skills["review"].Next
	if len(reviewRoutes) != 2 {
		t.Fatalf("len(review.Next) = %d, want 2", len(reviewRoutes))
	}
	if reviewRoutes[0].When != "<REVIEW_OK>" || reviewRoutes[0].Criteria != "品質基準を満たしている場合" || reviewRoutes[0].Skill != "<DONE>" {
		t.Errorf("review.Next[0] = %+v", reviewRoutes[0])
	}
	if reviewRoutes[1].When != "" || reviewRoutes[1].Criteria != "改善が必要な場合" || reviewRoutes[1].Skill != "impl" {
		t.Errorf("review.Next[1] = %+v", reviewRoutes[1])
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
		t.Error("Load() should return error for missing default_entrypoint")
	}
}

func TestLoadNoSkills(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`default_entrypoint: review
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

	content := []byte(`default_entrypoint: missing
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
		t.Error("Load() should return error when default_entrypoint not found in skills")
	}
}

func TestLoadRouteReferencesUnknownSkill(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`default_entrypoint: impl
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

	content := []byte(`default_entrypoint: impl
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

	content := []byte(`default_entrypoint: impl
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

func TestLoadMaxRestartsZeroDisablesAutoRestart(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`default_entrypoint: impl
max_restarts: 0
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

	if cfg.EffectiveMaxRestarts() != 0 {
		t.Errorf("EffectiveMaxRestarts = %d, want 0", cfg.EffectiveMaxRestarts())
	}
}

func TestLoadMaxRestartsNegativeIsRejected(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`default_entrypoint: impl
max_restarts: -1
skills:
  impl:
    next:
      - skill: "<DONE>"
`)
	if err := os.WriteFile(cfgFile, content, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Error("Load() should return error for negative max_restarts")
	}
}
