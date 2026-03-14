package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/executor"
	"github.com/takumiyoshikawa/skill-loop/internal/orchestrator"
	"github.com/takumiyoshikawa/skill-loop/internal/scheduler"
	"github.com/takumiyoshikawa/skill-loop/internal/session"
)

const defaultConfigFile = "skill-loop.yml"

func NewRunCmd() *cobra.Command {
	var maxIterations int
	var prompt string
	var entrypoint string
	var attach bool

	cmd := &cobra.Command{
		Use:   "run [config.yml]",
		Short: "Run a skill loop (detached by default)",
		Long: `Run a skill loop from a config file.

By default, this starts the orchestrator in a detached tmux session and returns immediately.
Use --attach to attach to that tmux session immediately.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := resolveConfigPath(args)
			if err != nil {
				return err
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if os.Getenv("SKILL_LOOP_SCHEDULE_CHILD") == "1" {
				sessionID := os.Getenv("SKILL_LOOP_SESSION_ID")
				if sessionID == "" {
					return fmt.Errorf("SKILL_LOOP_SESSION_ID is required in schedule child mode")
				}
				repoRoot := os.Getenv("SKILL_LOOP_SESSION_REPO_ROOT")
				if repoRoot == "" {
					repoRoot, err = session.ResolveRepoRoot("")
					if err != nil {
						return err
					}
				}
				return scheduler.Run(repoRoot, sessionID, cfg, maxIterations, prompt, entrypoint)
			}

			// Child mode for detached orchestrator process.
			if os.Getenv("SKILL_LOOP_RUN_CHILD") == "1" {
				runErr := orchestrator.RunWithObserver(
					cfg,
					maxIterations,
					prompt,
					entrypoint,
					&defaultExecutor{},
					newSessionRunObserver(os.Getenv("SKILL_LOOP_SESSION_REPO_ROOT"), os.Getenv("SKILL_LOOP_SESSION_ID")),
				)
				var blocked *orchestrator.BlockedError
				if errors.As(runErr, &blocked) {
					if err := persistBlockedSession(blocked); err != nil {
						return err
					}
					return nil
				}
				return runErr
			}

			childArgs := buildRunChildArgs(cfgPath, maxIterations, prompt, entrypoint)
			workflowName := cfg.EffectiveName(cfgPath)
			var meta *session.Metadata
			if cfg.Schedule != "" {
				meta, err = startDetachedScheduledRun(cfg, cfgPath, workflowName, childArgs, maxIterations, entrypoint)
			} else {
				meta, err = startDetachedRun(cfgPath, workflowName, childArgs)
			}
			if err != nil {
				return err
			}

			fmt.Printf("Started in background. run_id=%s\n", meta.ID)
			fmt.Printf("Attach: skill-loop sessions attach %s\n", meta.ID)
			fmt.Printf("Session: %s\n", filepath.Dir(meta.ScriptPath))
			fmt.Printf("Stdout: %s\n", meta.StdoutPath)
			fmt.Printf("Stderr: %s\n", meta.StderrPath)

			if attach {
				return session.Attach(meta)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&maxIterations, "max-iterations", 0, "Maximum number of loop iterations (overrides config; default 100)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Initial prompt passed to the first skill")
	cmd.Flags().StringVarP(&entrypoint, "entrypoint", "e", "", "Skill to start from (overrides config default_entrypoint)")
	cmd.Flags().BoolVar(&attach, "attach", false, "Attach to the detached run session immediately")

	return cmd
}

func resolveConfigPath(args []string) (string, error) {
	cfgPath := defaultConfigFile
	if len(args) > 0 {
		cfgPath = args[0]
	} else if _, err := os.Stat(defaultConfigFile); err != nil {
		return "", fmt.Errorf("no config file specified and %s not found in current directory", defaultConfigFile)
	}
	absPath, err := filepath.Abs(cfgPath)
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	cfgPath = absPath
	return cfgPath, nil
}

func buildRunChildArgs(cfgPath string, maxIterations int, prompt string, entrypoint string) []string {
	args := []string{"run", cfgPath}
	if maxIterations > 0 {
		args = append(args, "--max-iterations", strconv.Itoa(maxIterations))
	}
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}
	if entrypoint != "" {
		args = append(args, "--entrypoint", entrypoint)
	}
	return args
}

func startDetachedRun(cfgPath string, workflowName string, childArgs []string) (*session.Metadata, error) {
	return startDetachedSession(cfgPath, workflowName, childArgs, map[string]string{
		"SKILL_LOOP_RUN_CHILD": "1",
	})
}

func startDetachedScheduledRun(cfg *config.Config, cfgPath string, workflowName string, childArgs []string, maxIterations int, entrypoint string) (*session.Metadata, error) {
	meta, err := startDetachedSession(cfgPath, workflowName, childArgs, map[string]string{
		"SKILL_LOOP_SCHEDULE_CHILD": "1",
	})
	if err != nil {
		return nil, err
	}

	effectiveMaxIterations := maxIterations
	if effectiveMaxIterations <= 0 {
		effectiveMaxIterations = cfg.MaxIterations
	}
	if effectiveMaxIterations <= 0 {
		effectiveMaxIterations = orchestrator.DefaultMaxIterations
	}

	schedule, err := cron.ParseStandard(cfg.Schedule)
	if err != nil {
		return nil, fmt.Errorf("parse schedule: %w", err)
	}
	nextRun := schedule.Next(time.Now())

	meta.Schedule = cfg.Schedule
	meta.Status = session.StatusScheduled
	meta.NextRun = &nextRun
	meta.CurrentIteration = 0
	meta.MaxIterations = effectiveMaxIterations
	meta.CurrentSkill = ""
	meta.LastError = ""
	meta.EndedAt = nil
	meta.ConfigPath = cfgPath

	if err := session.Save(meta); err != nil {
		return nil, fmt.Errorf("persist scheduled session metadata: %w", err)
	}

	return meta, nil
}

func startDetachedSession(cfgPath string, workflowName string, childArgs []string, childEnv map[string]string) (*session.Metadata, error) {
	configDir := filepath.Dir(cfgPath)
	if configDir == "" || configDir == "." {
		var err error
		configDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	repoRoot, err := session.ResolveRepoRoot(configDir)
	if err != nil {
		return nil, err
	}

	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	exePath, err = resolveDetachedBinary(exePath)
	if err != nil {
		return nil, err
	}

	command := append([]string{"env"}, envAssignments(childEnv)...)
	command = append(command, exePath)
	command = append(command, childArgs...)
	workingDir := configDir

	meta, err := session.New(repoRoot, workingDir, workflowName, "orchestrator", "skill-loop", command, 0, 0)
	if err != nil {
		return nil, err
	}
	meta.ConfigPath = cfgPath

	if err := session.Start(meta); err != nil {
		cleanupErr := os.RemoveAll(filepath.Dir(meta.ScriptPath))
		if cleanupErr != nil {
			return nil, fmt.Errorf("start detached run: %w (cleanup failed: %v)", err, cleanupErr)
		}
		return nil, err
	}

	return meta, nil
}

func envAssignments(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	assignments := make([]string, 0, len(env))
	for key, value := range env {
		assignments = append(assignments, key+"="+value)
	}
	sort.Strings(assignments)
	return assignments
}

type defaultExecutor struct{}

func (d *defaultExecutor) ExecuteSkill(name string, agent config.Agent, input string, opts executor.ExecutionOptions) (*executor.SkillResult, error) {
	return executor.ExecuteSkill(name, agent, input, opts)
}

func (d *defaultExecutor) RouteSkillOutput(skillName string, router config.Agent, output string, routes []config.Route, opts executor.ExecutionOptions) (*executor.RouterDecision, error) {
	return executor.RouteSkillOutput(skillName, router, output, routes, opts)
}

type sessionRunObserver struct {
	repoRoot  string
	sessionID string
}

func newSessionRunObserver(repoRoot string, sessionID string) orchestrator.RunObserver {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	return &sessionRunObserver{repoRoot: repoRoot, sessionID: sessionID}
}

func (o *sessionRunObserver) IterationStarted(iteration int, maxIterations int, skill string) {
	_ = o.update(func(meta *session.Metadata) {
		meta.Status = session.StatusRunning
		meta.CurrentIteration = iteration
		meta.MaxIterations = maxIterations
		meta.CurrentSkill = skill
	})
}

func (o *sessionRunObserver) SkillCompleted(iteration int, maxIterations int, skill string, stdout string) {
	_ = o.update(func(meta *session.Metadata) {
		meta.Status = session.StatusRunning
		meta.CurrentIteration = iteration
		meta.MaxIterations = maxIterations
		meta.CurrentSkill = skill
		meta.LastSkillOutput = stdout
	})
}

func (o *sessionRunObserver) update(apply func(*session.Metadata)) error {
	meta, err := session.LoadByID(o.repoRoot, o.sessionID)
	if err != nil {
		return err
	}
	apply(meta)
	return session.Save(meta)
}

func resolveDetachedBinary(exePath string) (string, error) {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("resolve absolute executable path: %w", err)
	}

	if !strings.Contains(exePath, string(filepath.Separator)+"go-build") {
		return exePath, nil
	}

	installedPath, err := exec.LookPath("skill-loop")
	if err != nil {
		return "", fmt.Errorf("detached runs require an installed skill-loop binary on PATH when launched via go run: %w", err)
	}
	installedPath, err = filepath.Abs(installedPath)
	if err != nil {
		return "", fmt.Errorf("resolve installed skill-loop path: %w", err)
	}
	return installedPath, nil
}

func persistBlockedSession(blocked *orchestrator.BlockedError) error {
	sessionID := os.Getenv("SKILL_LOOP_SESSION_ID")
	if sessionID == "" {
		return fmt.Errorf("persist blocked session: SKILL_LOOP_SESSION_ID is required")
	}

	repoRoot := os.Getenv("SKILL_LOOP_SESSION_REPO_ROOT")
	if repoRoot == "" {
		var err error
		repoRoot, err = session.ResolveRepoRoot("")
		if err != nil {
			return fmt.Errorf("persist blocked session: %w", err)
		}
	}

	meta, err := session.LoadByID(repoRoot, sessionID)
	if err != nil {
		return fmt.Errorf("persist blocked session: %w", err)
	}

	now := time.Now().UTC()
	meta.Status = session.StatusBlocked
	meta.EndedAt = &now
	meta.BlockReason = blocked.Reason
	meta.ResumeSkill = blocked.Skill
	meta.ResumePrompt = blocked.Prompt
	meta.LastError = ""

	if err := session.Save(meta); err != nil {
		return fmt.Errorf("persist blocked session: %w", err)
	}
	return nil
}
