package cli

import (
	"io"

	"github.com/firety/firety/internal/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(application *app.App, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "firety",
		Short:         "Lint local SKILL.md packages",
		Long:          "Firety is a lightweight open-source CLI for linting local SKILL.md packages and their referenced resources.",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(
		newSkillCommand(application),
		hideCommand(newArtifactCommand()),
		hideCommand(newBenchmarkCommand(application)),
		hideCommand(newEvidenceCommand(application)),
		hideCommand(newFreshnessCommand()),
		hideCommand(newPublishCommand(application)),
		hideCommand(newProvenanceCommand()),
		hideCommand(newReadinessCommand(application)),
		hideCommand(newWorkspaceCommand(application)),
		hideCommand(newMCPCommand(application)),
		hideCommand(newAgentCommand(application)),
		hideCommand(newVersionCommand(application)),
	)

	return root
}

func hideCommand(cmd *cobra.Command) *cobra.Command {
	cmd.Hidden = true
	return cmd
}
