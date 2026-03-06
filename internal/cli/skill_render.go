package cli

import (
	"fmt"
	"io"

	"github.com/firety/firety/internal/render"
	"github.com/spf13/cobra"
)

type skillRenderOptions struct {
	mode string
}

func newSkillRenderCommand() *cobra.Command {
	options := skillRenderOptions{
		mode: string(render.ModeFullReport),
	}

	cmd := &cobra.Command{
		Use:   "render <artifact>",
		Short: "Render Firety artifacts into reviewer-friendly reports",
		Long:  "Render an existing Firety artifact into a PR comment, CI summary, or fuller human-readable report.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			output, err := render.RenderArtifact(args[0], render.Mode(options.mode))
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillRenderedArtifact(cmd.OutOrStdout(), output); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(
		&options.mode,
		"render",
		string(render.ModeFullReport),
		"Render mode: pr-comment, ci-summary, or full-report",
	)

	return cmd
}

func (o skillRenderOptions) Validate() error {
	if _, err := render.ParseMode(o.mode); err != nil {
		return err
	}
	return nil
}

func writeSkillRenderedArtifact(w io.Writer, output string) error {
	if _, err := fmt.Fprint(w, output); err != nil {
		return err
	}
	return nil
}
