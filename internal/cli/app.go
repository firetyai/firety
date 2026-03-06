package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/capability"
	"github.com/spf13/cobra"
)

const (
	ExitCodeOK      = 0
	ExitCodeLint    = 1
	ExitCodeRuntime = 2
)

type commandExitError struct {
	code int
	err  error
}

func (e *commandExitError) Error() string {
	if e.err == nil {
		return ""
	}

	return e.err.Error()
}

func Execute(application *app.App, stdout, stderr io.Writer, args ...string) (int, error) {
	cmd := NewRootCommand(application, stdout, stderr)
	if len(args) > 0 {
		cmd.SetArgs(args)
	}

	if err := cmd.Execute(); err != nil {
		var coded *commandExitError
		if errors.As(err, &coded) {
			return coded.code, coded.err
		}

		return ExitCodeRuntime, err
	}

	return ExitCodeOK, nil
}

func runPlaceholder(application *app.App, kind capability.Kind) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), application.Services.Placeholder.Message(kind))
		return err
	}
}

func newExitError(code int, err error) error {
	return &commandExitError{code: code, err: err}
}
