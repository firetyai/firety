package testutil

import (
	"bytes"
	"io"
	"testing"

	"github.com/spf13/cobra"
)

type CommandBuilder func(stdout, stderr io.Writer) *cobra.Command

func ExecuteCommand(t *testing.T, build CommandBuilder, args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := build(&stdout, &stderr)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return stdout.String(), stderr.String(), err
}
