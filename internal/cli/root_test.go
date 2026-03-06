package cli_test

import (
	"bytes"
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

	for _, expected := range []string{"skill"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected help output to contain %q, got %q", expected, stdout)
		}
	}
	for _, hidden := range []string{"artifact", "benchmark", "evidence", "freshness", "publish", "provenance", "readiness", "workspace", "mcp", "agent"} {
		if strings.Contains(stdout, hidden) {
			t.Fatalf("expected help output to hide %q, got %q", hidden, stdout)
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

	stdout, stderr, err := testutil.ExecuteCommand(t, newTestCommand, "skill", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "lint") {
		t.Fatalf("expected help output to contain lint, got %q", stdout)
	}
	for _, hidden := range []string{"attest", "baseline", "compatibility", "plan", "analyze", "eval", "eval-compare", "gate", "compare", "render", "rules"} {
		if strings.Contains(stdout, hidden) {
			t.Fatalf("expected help output to hide %q, got %q", hidden, stdout)
		}
	}
}

func TestSkillCommandDefaultsToLint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	stdout, stderr, code, err := executeSkill(t, root)
	if err != nil {
		t.Fatalf("expected no runtime error, got %v", err)
	}
	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "valid") && !strings.Contains(stdout, "0 error") && !strings.Contains(stdout, "warning") {
		t.Fatalf("expected lint-style output, got %q", stdout)
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

func executeSkill(t *testing.T, root string, args ...string) (string, string, int, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	commandArgs := append([]string{"skill", root}, args...)
	code, err := cli.Execute(newTestApplication(), &stdout, &stderr, commandArgs...)
	return stdout.String(), stderr.String(), code, err
}
