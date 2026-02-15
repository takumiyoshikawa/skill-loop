package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/orchestrator"
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

			// Child mode for detached orchestrator process.
			if os.Getenv("SKILL_LOOP_RUN_CHILD") == "1" {
				return orchestrator.Run(cfg, maxIterations, prompt, entrypoint)
			}

			childArgs := buildRunChildArgs(cfgPath, maxIterations, prompt, entrypoint)
			meta, err := startDetachedRun(childArgs)
			if err != nil {
				return err
			}

			fmt.Printf("Started in background. run_id=%s\n", meta.ID)
			fmt.Printf("Attach: skill-loop sessions attach %s\n", meta.ID)

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

func startDetachedRun(childArgs []string) (*session.Metadata, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, err := session.ResolveRepoRoot(wd)
	if err != nil {
		return nil, err
	}

	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute executable path: %w", err)
	}

	command := append([]string{"env", "SKILL_LOOP_RUN_CHILD=1", exePath}, childArgs...)
	workingDir := wd
	// `go run` builds into a temporary location; use source invocation for detached child.
	if strings.Contains(exePath, string(filepath.Separator)+"go-build") {
		command = append([]string{"env", "SKILL_LOOP_RUN_CHILD=1", "go", "run", "./cmd/skill-loop"}, childArgs...)
		workingDir = repoRoot
	}

	meta, err := session.New(repoRoot, workingDir, "orchestrator", "skill-loop", command, 0, 0)
	if err != nil {
		return nil, err
	}

	if err := session.Start(meta); err != nil {
		cleanupErr := os.RemoveAll(filepath.Dir(meta.ScriptPath))
		if cleanupErr != nil {
			return nil, fmt.Errorf("start detached run: %w (cleanup failed: %v)", err, cleanupErr)
		}
		return nil, err
	}

	return meta, nil
}
