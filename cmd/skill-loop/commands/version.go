package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/takumiyoshikawa/skill-loop/internal/version"
)

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of skill-loop",
		Long:  `Print the version number of skill-loop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(version.Info())
			return nil
		},
	}
}
