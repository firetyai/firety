package cli

import (
	"io"

	"github.com/firety/firety/internal/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(application *app.App, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "firety",
		Short:         "Test and compare reusable agent capabilities across tools",
		Long:          "Firety is an open-source CLI for testing and comparing reusable agent capabilities such as Skills, MCP servers, and agent integrations across multiple tools.",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(
		newArtifactCommand(),
		newBenchmarkCommand(application),
		newEvidenceCommand(application),
		newFreshnessCommand(),
		newPublishCommand(application),
		newProvenanceCommand(),
		newReadinessCommand(application),
		newSkillCommand(application),
		newWorkspaceCommand(application),
		newMCPCommand(application),
		newAgentCommand(application),
		newVersionCommand(application),
	)

	return root
}
