package service

import (
	"strings"
	"testing"

	"github.com/firety/firety/internal/testutil"
)

func TestLoadRoutingEvalSuite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	suite, err := loadRoutingEvalSuite(root + "/evals/routing.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if suite.Name != "curated-routing-suite" {
		t.Fatalf("unexpected suite metadata: %#v", suite)
	}
	if len(suite.Cases) != 4 {
		t.Fatalf("expected four cases, got %#v", suite)
	}
}

func TestLoadRoutingEvalSuiteRejectsInvalidCase(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"evals/routing.json": strings.Join([]string{
			"{",
			`  "schema_version": "1",`,
			`  "name": "invalid",`,
			`  "cases": [`,
			`    {"id": "broken", "prompt": "", "expectation": "maybe"}`,
			"  ]",
			"}",
		}, "\n"),
	})

	_, err := loadRoutingEvalSuite(root + "/evals/routing.json")
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestSkillEvalServiceRequiresRunner(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"SKILL.md":           testutil.RoutingEvalPortableSkillMarkdown(),
		"evals/routing.json": testutil.RoutingEvalFixtureJSON(),
	})

	report, err := NewSkillEvalService().Evaluate(root, SkillEvalOptions{})
	if err == nil {
		t.Fatalf("expected runner error, got report %#v", report)
	}
	if !strings.Contains(err.Error(), "routing eval runner is not configured") {
		t.Fatalf("expected missing-runner error, got %v", err)
	}
}
