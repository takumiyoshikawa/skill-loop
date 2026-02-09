package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yoshikawatakumi/skill-loop/internal/jsonschema"
)

func NewSchemaCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Generate JSON Schema for skill-loop.yml files",
		Long: `Generate a JSON Schema that can be used for IDE autocomplete and validation
of skill-loop.yml configuration files.

The generated schema can be used with yaml-language-server by adding a comment
at the top of your skill-loop.yml file:

  # yaml-language-server: $schema=https://raw.githubusercontent.com/yoshikawatakumi/skill-loop/main/schema.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaBytes, err := jsonschema.Generate()
			if err != nil {
				return fmt.Errorf("generating schema: %w", err)
			}

			if outputFile != "" {
				if dir := filepath.Dir(outputFile); dir != "." {
					if err := os.MkdirAll(dir, 0o750); err != nil {
						return fmt.Errorf("creating directory %s: %w", dir, err)
					}
				}
				if err := os.WriteFile(outputFile, schemaBytes, 0o600); err != nil {
					return fmt.Errorf("writing schema: %w", err)
				}
				fmt.Printf("JSON Schema written to %s\n", outputFile)
			} else {
				fmt.Println(string(schemaBytes))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")

	return cmd
}
