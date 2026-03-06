package artifact_test

import (
	"encoding/json"
	"testing"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/gate"
)

func TestBuildSkillGateArtifactDeterministic(t *testing.T) {
	t.Parallel()

	version := app.VersionInfo{
		Version: "test-version",
		Commit:  "abc1234",
		Date:    "2026-03-06T00:00:00Z",
	}
	result := gate.Result{
		Decision: gate.DecisionFail,
		Summary:  "Gate failed: 1 blocking criterion violation(s).",
		Criteria: gate.Criteria{
			MaxLintErrors: intPointer(0),
		},
		BlockingReasons: []gate.Reason{
			{
				Code:           "gate.lint-errors-exceeded",
				Title:          "Lint errors exceed policy",
				Summary:        "The candidate has 1 lint error(s), above the allowed maximum of 0.",
				RelatedRuleIDs: []string{"skill.missing-title"},
			},
		},
	}

	left := artifact.BuildSkillGateArtifact(version, result, artifact.SkillGateArtifactOptions{
		Format:     "json",
		Target:     "candidate",
		Profile:    "generic",
		Strictness: "default",
	}, 1)
	right := artifact.BuildSkillGateArtifact(version, result, artifact.SkillGateArtifactOptions{
		Format:     "json",
		Target:     "candidate",
		Profile:    "generic",
		Strictness: "default",
	}, 1)

	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("marshal left artifact: %v", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("marshal right artifact: %v", err)
	}

	if string(leftJSON) != string(rightJSON) {
		t.Fatalf("expected deterministic gate artifact, left=%s right=%s", leftJSON, rightJSON)
	}
	if left.SchemaVersion != artifact.SkillGateArtifactSchemaVersion {
		t.Fatalf("expected schema version %q, got %#v", artifact.SkillGateArtifactSchemaVersion, left)
	}
	if left.ArtifactType != "firety.skill-quality-gate" {
		t.Fatalf("expected gate artifact type, got %#v", left)
	}
}

func intPointer(value int) *int {
	return &value
}
