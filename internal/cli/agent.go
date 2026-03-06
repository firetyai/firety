package cli

import (
	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/capability"
	"github.com/spf13/cobra"
)

func newAgentCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "agent",
		Short: "Work with agent integrations",
		Long:  "Agent integration commands will live here as Firety grows. The current implementation is scaffolding only.",
		Args:  cobra.NoArgs,
		RunE:  runPlaceholder(application, capability.KindAgent),
	}
}
