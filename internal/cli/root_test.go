package cli_test

import (
	"io"
	"strings"
	"testing"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/testutil"
	"github.com/spf13/cobra"
)

func newTestCommand(stdout, stderr io.Writer) *cobra.Command {
	return cli.NewRootCommand(
		app.New(app.VersionInfo{
			Version: "test-version",
			Commit:  "abc1234",
			Date:    "2026-03-06T00:00:00Z",
		}),
		stdout,
		stderr,
	)
}

func TestRootCommandHelp(t *testing.T) {
	stdout, stderr, err := testutil.ExecuteCommand(t, newTestCommand, "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	for _, expected := range []string{"benchmark", "skill", "mcp", "agent", "version"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected help output to contain %q, got %q", expected, stdout)
		}
	}
}

func TestPlaceholderCommands(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "mcp",
			args:     []string{"mcp"},
			expected: "mcp command is scaffolded but not implemented yet\n",
		},
		{
			name:     "agent",
			args:     []string{"agent"},
			expected: "agent command is scaffolded but not implemented yet\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, err := testutil.ExecuteCommand(t, newTestCommand, tc.args...)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if stdout != tc.expected {
				t.Fatalf("expected stdout %q, got %q", tc.expected, stdout)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestSkillCommandHelp(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := testutil.ExecuteCommand(t, newTestCommand, "skill")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	for _, expected := range []string{"lint", "plan", "analyze", "eval", "eval-compare", "compare", "render", "rules"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected help output to contain %q, got %q", expected, stdout)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := testutil.ExecuteCommand(t, newTestCommand, "version")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := "version: test-version\ncommit: abc1234\nbuilt: 2026-03-06T00:00:00Z\n"
	if stdout != expected {
		t.Fatalf("expected stdout %q, got %q", expected, stdout)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
