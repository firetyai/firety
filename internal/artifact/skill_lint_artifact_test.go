package artifact_test

import (
	"encoding/json"
	"testing"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
)

func TestBuildSkillLintArtifactIsDeterministic(t *testing.T) {
	t.Parallel()

	report := lint.NewReport("/tmp/skill")
	report.AddRule(lint.RuleMissingExamples, "no obvious examples section or usage examples found", "SKILL.md", 0)
	report.AddRule(lint.RuleMissingUsageGuidance, "no obvious invocation guidance or input expectations found", "SKILL.md", 0)
	report.ApplyStrictness(lint.StrictnessPedantic)

	fixResult := service.SkillFixResult{
		Applied: []service.AppliedSkillFix{
			{
				Rule:    lint.RuleMissingTitle,
				Path:    "SKILL.md",
				Message: "inserted a top-level title derived from the front matter name",
			},
		},
	}

	options := artifact.SkillLintArtifactOptions{
		Format:      "json",
		Profile:     "generic",
		Strictness:  "pedantic",
		FailOn:      "warnings",
		Explain:     true,
		RoutingRisk: true,
		Fix:         true,
	}
	version := app.VersionInfo{
		Version: "test-version",
		Commit:  "abc1234",
		Date:    "2026-03-06T00:00:00Z",
	}

	left := artifact.BuildSkillLintArtifact(version, report, fixResult, options, 1)
	right := artifact.BuildSkillLintArtifact(version, report, fixResult, options, 1)

	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("marshal left artifact: %v", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("marshal right artifact: %v", err)
	}

	if string(leftJSON) != string(rightJSON) {
		t.Fatalf("expected deterministic artifact, left=%s right=%s", leftJSON, rightJSON)
	}

	if left.SchemaVersion != artifact.SkillLintArtifactSchemaVersion {
		t.Fatalf("expected schema version %q, got %#v", artifact.SkillLintArtifactSchemaVersion, left)
	}
	if left.Run.Strictness != "pedantic" {
		t.Fatalf("expected strictness metadata, got %#v", left.Run)
	}
	if left.Summary.PassesFailPolicy {
		t.Fatalf("expected fail-policy summary to be false for warnings policy, got %#v", left.Summary)
	}
	if left.Summary.AppliedFixCount != 1 {
		t.Fatalf("expected applied fix count, got %#v", left.Summary)
	}
	if len(left.RuleCatalog) != 3 {
		t.Fatalf("expected referenced rules for findings and fix, got %#v", left.RuleCatalog)
	}
	if left.Fingerprint == "" {
		t.Fatalf("expected fingerprint, got %#v", left)
	}
	if left.RoutingRisk == nil || left.RoutingRisk.OverallRisk == "" {
		t.Fatalf("expected routing risk in artifact, got %#v", left)
	}
}
