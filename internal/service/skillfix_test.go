package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/firety/firety/internal/testutil"
)

func TestSkillFixerAppliesMissingTitleFix(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md": strings.Join([]string{
			"---",
			"name: Example Skill",
			"description: Validates local skill directories before publishing them.",
			"---",
			"## When To Use",
			"",
			"Use this skill when you need to validate a local skill directory before publishing changes.",
			"",
			"## Usage",
			"",
			"Run `firety skill lint .` from the skill root.",
			"",
			"## Limitations",
			"",
			"Do not use this skill for remote repositories or provider-specific integrations.",
			"",
			"## Examples",
			"",
			"Request: Validate this local skill directory before publishing it.",
			"Invocation: Run `firety skill lint . --format json` from the skill root.",
			"Result: Review the reported findings before publishing.",
		}, "\n"),
	})

	result, err := service.NewSkillFixer().Apply(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Applied) != 1 {
		t.Fatalf("expected one applied fix, got %#v", result)
	}
	if result.Applied[0].Rule.ID != lint.RuleMissingTitle.ID {
		t.Fatalf("expected missing-title fix, got %#v", result.Applied[0])
	}

	content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	expectedPrefix := strings.Join([]string{
		"---",
		"name: Example Skill",
		"description: Validates local skill directories before publishing them.",
		"---",
		"# Example Skill",
		"",
	}, "\n")
	if !strings.HasPrefix(string(content), expectedPrefix) {
		t.Fatalf("expected inserted title, got %q", string(content))
	}
}

func TestSkillFixerDoesNotRewriteSupportedContentWithoutNeed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, testutil.ValidSkillFiles())

	before, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	result, err := service.NewSkillFixer().Apply(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("expected no fixes, got %#v", result)
	}

	after, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if string(after) != string(before) {
		t.Fatalf("expected no file changes")
	}
}
