package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")

	content := []byte(`name: nightly-review
default_entrypoint: impl
router:
  runtime: codex
  model: gpt-5.4
skills:
  impl:
    agent:
      runtime: claude
      model: claude-sonnet-4.6
    next:
      - id: keep-implementing
        criteria: "まだ作業が必要な場合"
        skill: impl
      - id: send-review
        criteria: "実装が完了した場合"
        skill: review
  review:
    next:
      - id: approve
        criteria: "品質基準を満たしている場合"
        done: true
      - id: rework
        criteria: "改善が必要な場合"
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
	if cfg.Name != "nightly-review" {
		t.Errorf("Name = %q, want %q", cfg.Name, "nightly-review")
	}
	if cfg.Router.Runtime != "codex" {
		t.Errorf("Router.Runtime = %q, want %q", cfg.Router.Runtime, "codex")
	}
	if cfg.Skills["impl"].Agent.Model != "claude-sonnet-4.6" {
		t.Errorf("Skills[impl].Agent.Model = %q, want %q", cfg.Skills["impl"].Agent.Model, "claude-sonnet-4.6")
	}
	if cfg.IdleTimeoutSeconds != 900 {
		t.Errorf("IdleTimeoutSeconds = %d, want 900", cfg.IdleTimeoutSeconds)
	}
	if cfg.EffectiveMaxRestarts() != 2 {
		t.Errorf("EffectiveMaxRestarts = %d, want 2", cfg.EffectiveMaxRestarts())
	}
	if got := cfg.EffectiveName(cfgFile); got != "nightly-review" {
		t.Errorf("EffectiveName() = %q, want %q", got, "nightly-review")
	}

	implRoutes := cfg.Skills["impl"].Next
	if len(implRoutes) != 2 {
		t.Fatalf("len(impl.Next) = %d, want 2", len(implRoutes))
	}
	if implRoutes[0].ID != "keep-implementing" || implRoutes[0].Skill != "impl" || implRoutes[0].Done {
		t.Errorf("impl.Next[0] = %+v", implRoutes[0])
	}
	if implRoutes[1].ID != "send-review" || implRoutes[1].Skill != "review" || implRoutes[1].Done {
		t.Errorf("impl.Next[1] = %+v", implRoutes[1])
	}

	reviewRoutes := cfg.Skills["review"].Next
	if len(reviewRoutes) != 2 {
		t.Fatalf("len(review.Next) = %d, want 2", len(reviewRoutes))
	}
	if reviewRoutes[0].ID != "approve" || !reviewRoutes[0].Done || reviewRoutes[0].Skill != "" {
		t.Errorf("review.Next[0] = %+v", reviewRoutes[0])
	}
	if reviewRoutes[1].ID != "rework" || reviewRoutes[1].Skill != "impl" || reviewRoutes[1].Done {
		t.Errorf("review.Next[1] = %+v", reviewRoutes[1])
	}
}

func TestLoadMissingEntrypoint(t *testing.T) {
	_, err := Load(writeConfig(t, `skills:
  review:
    next:
      - id: finish
        done: true
`))
	if err == nil {
		t.Error("Load() should return error for missing default_entrypoint")
	}
}

func TestLoadNoSkills(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: review
`))
	if err == nil {
		t.Error("Load() should return error when no skills defined")
	}
}

func TestLoadEntrypointNotInSkills(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: missing
skills:
  review:
    next:
      - id: finish
        done: true
`))
	if err == nil {
		t.Error("Load() should return error when default_entrypoint not found in skills")
	}
}

func TestLoadRouteReferencesUnknownSkill(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: missing
        skill: nonexistent
`))
	if err == nil {
		t.Error("Load() should return error when route references unknown skill")
	}
}

func TestLoadRouteRequiresID(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - skill: impl
`))
	if err == nil {
		t.Error("Load() should return error when route id is missing")
	}
}

func TestLoadRejectsLegacyWhen(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: keep-going
        when: "<OLD>"
        skill: impl
`))
	if err == nil || !strings.Contains(err.Error(), "deprecated when") {
		t.Fatalf("Load() error = %v, want deprecated when error", err)
	}
}

func TestLoadRejectsDeprecatedDoneSentinel(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: finish
        skill: "<DONE>"
`))
	if err == nil || !strings.Contains(err.Error(), "deprecated <DONE>") {
		t.Fatalf("Load() error = %v, want deprecated <DONE> error", err)
	}
}

func TestLoadRejectsDuplicateRouteID(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
router:
  runtime: codex
skills:
  impl:
    next:
      - id: choose
        criteria: "keep working"
        skill: impl
      - id: choose
        criteria: "ship it"
        done: true
`))
	if err == nil {
		t.Error("Load() should return error for duplicate route id")
	}
}

func TestLoadRejectsMissingCriteriaWhenMultipleRoutes(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
router:
  runtime: codex
skills:
  impl:
    next:
      - id: keep-working
        skill: impl
      - id: finish
        criteria: "done"
        done: true
`))
	if err == nil {
		t.Error("Load() should return error when criteria is missing for multi-route skill")
	}
}

func TestLoadRejectsRouteWithDoneAndSkill(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: bad
        done: true
        skill: impl
`))
	if err == nil {
		t.Error("Load() should return error when route sets both done and skill")
	}
}

func TestLoadRejectsRouteWithDoneAndBlocked(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: bad
        done: true
        blocked: true
`))
	if err == nil {
		t.Error("Load() should return error when route sets both done and blocked")
	}
}

func TestLoadRejectsBlockedRouteWithoutSkill(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: need-human
        blocked: true
`))
	if err == nil {
		t.Error("Load() should return error when blocked route has no skill")
	}
}

func TestLoadAllowsBlockedRouteWithSkill(t *testing.T) {
	cfg, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: need-human
        blocked: true
        skill: confirm
  confirm:
    next:
      - id: done
        done: true
`))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.Skills["impl"].Next[0].Blocked {
		t.Fatal("expected blocked route to be preserved")
	}
}

func TestLoadRequiresRouterForMultiRouteSkills(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    next:
      - id: keep-working
        criteria: "more work"
        skill: impl
      - id: finish
        criteria: "done"
        done: true
`))
	if err == nil {
		t.Error("Load() should return error when router is missing for multi-route skills")
	}
}

func TestLoadAllowsSingleRouteWithoutRouter(t *testing.T) {
	cfg, err := Load(writeConfig(t, `default_entrypoint: hello
skills:
  hello:
    next:
      - id: finish
        done: true
`))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.Skills["hello"].Next[0].Done {
		t.Fatal("expected single route to be done")
	}
}

func TestLoadValidSchedule(t *testing.T) {
	cfg, err := Load(writeConfig(t, `schedule: "0 9 * * *"
default_entrypoint: impl
skills:
  impl:
    next:
      - id: finish
        done: true
`))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Schedule != "0 9 * * *" {
		t.Errorf("Schedule = %q, want %q", cfg.Schedule, "0 9 * * *")
	}
}

func TestLoadInvalidSchedule(t *testing.T) {
	_, err := Load(writeConfig(t, `schedule: "not-a-cron"
default_entrypoint: impl
skills:
  impl:
    next:
      - id: finish
        done: true
`))
	if err == nil {
		t.Error("Load() should return error for invalid schedule")
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestEffectiveNameFallsBackToConfigFilename(t *testing.T) {
	cfg := &Config{}
	got := cfg.EffectiveName("/tmp/My Workflow.yml")
	if got != "my-workflow" {
		t.Fatalf("EffectiveName() = %q, want %q", got, "my-workflow")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	_, err := Load(writeConfig(t, "this is not valid: yaml: ["))
	if err == nil {
		t.Error("Load() should return error for invalid YAML")
	}
}

func TestLoadInvalidAgent(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    agent:
      runtime: unknown
    next:
      - id: finish
        done: true
`))
	if err == nil {
		t.Error("Load() should return error for unsupported skill agent")
	}
}

func TestLoadInvalidRouterAgent(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
router:
  runtime: unknown
skills:
  impl:
    next:
      - id: keep-working
        criteria: "continue"
        skill: impl
      - id: finish
        criteria: "done"
        done: true
`))
	if err == nil {
		t.Error("Load() should return error for unsupported router agent")
	}
}

func TestLoadLegacyAgentString(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
skills:
  impl:
    agent: codex
    next:
      - id: finish
        done: true
`))
	if err == nil {
		t.Error("Load() should return error for legacy string agent format")
	}
}

func TestLoadMaxRestartsZeroDisablesAutoRestart(t *testing.T) {
	cfg, err := Load(writeConfig(t, `default_entrypoint: impl
max_restarts: 0
skills:
  impl:
    next:
      - id: finish
        done: true
`))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.EffectiveMaxRestarts() != 0 {
		t.Errorf("EffectiveMaxRestarts = %d, want 0", cfg.EffectiveMaxRestarts())
	}
}

func TestLoadMaxRestartsNegativeIsRejected(t *testing.T) {
	_, err := Load(writeConfig(t, `default_entrypoint: impl
max_restarts: -1
skills:
  impl:
    next:
      - id: finish
        done: true
`))
	if err == nil {
		t.Error("Load() should return error for negative max_restarts")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	cfgFile := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(cfgFile, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return cfgFile
}
