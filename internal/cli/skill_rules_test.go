package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/domain/lint"
)

func TestSkillRulesCommandTextOutput(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		"skill",
		"rules",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	output := stdout.String()
	for _, expected := range []string{
		"Firety lint rules",
		"Structure",
		"Metadata / Spec",
		"Portability",
		"skill.target-not-found [error]",
	} {
		if !bytes.Contains(stdout.Bytes(), []byte(expected)) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestSkillRulesCommandJSONOutput(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		"skill",
		"rules",
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if code != cli.ExitCodeOK {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeOK, code)
	}

	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var payload struct {
		Rules []lint.Rule `json:"rules"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json, got %v; output=%q", err, stdout.String())
	}

	if len(payload.Rules) != len(lint.AllRules()) {
		t.Fatalf("expected %d rules, got %d", len(lint.AllRules()), len(payload.Rules))
	}

	for index, rule := range lint.AllRules() {
		if payload.Rules[index].ID != rule.ID {
			t.Fatalf("expected rule %d to be %q, got %#v", index, rule.ID, payload.Rules[index])
		}
	}
}

func TestSkillRulesCommandInvalidFormat(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code, err := cli.Execute(
		newTestApplication(),
		&stdout,
		&stderr,
		"skill",
		"rules",
		"--format",
		"xml",
	)
	if err == nil {
		t.Fatalf("expected an error")
	}

	if code != cli.ExitCodeRuntime {
		t.Fatalf("expected exit code %d, got %d", cli.ExitCodeRuntime, code)
	}

	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	if err.Error() != `invalid format "xml": must be one of text, json` {
		t.Fatalf("unexpected error %q", err.Error())
	}
}
