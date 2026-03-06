package cli

import (
	"fmt"

	"github.com/firety/firety/internal/app"
	"github.com/spf13/cobra"
)

func newVersionCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"version: %s\ncommit: %s\nbuilt: %s\n",
				application.Version.Version,
				application.Version.Commit,
				application.Version.Date,
			)
			return err
		},
	}
}
