package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill-loop",
		Short: "An agentic skill orchestrator powered by Claude Code",
		Long: `skill-loop orchestrates multiple AI skills in a loop,
where each skill can delegate to the next based on its output.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(NewRunCmd())
	cmd.AddCommand(NewSchemaCmd())

	return cmd
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
