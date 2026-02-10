package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/orchestrator"
)

const defaultConfigFile = "skill-loop.yml"

func NewRunCmd() *cobra.Command {
	var maxIterations int
	var prompt string
	var entrypoint string

	cmd := &cobra.Command{
		Use:   "run [config.yml]",
		Short: "Run a skill loop from a config file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := defaultConfigFile
			if len(args) > 0 {
				cfgPath = args[0]
			} else if _, err := os.Stat(defaultConfigFile); err != nil {
				return fmt.Errorf("no config file specified and %s not found in current directory", defaultConfigFile)
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			return orchestrator.Run(cfg, maxIterations, prompt, entrypoint)
		},
	}

	cmd.Flags().IntVar(&maxIterations, "max-iterations", 0, "Maximum number of loop iterations (overrides config; default 100)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Initial prompt passed to the first skill")
	cmd.Flags().StringVarP(&entrypoint, "entrypoint", "e", "", "Skill to start from (overrides config default_entrypoint)")

	return cmd
}
