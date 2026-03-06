package artifact_test

import (
	"encoding/json"
	"testing"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/benchmark"
)

func TestBuildBenchmarkArtifactIsDeterministic(t *testing.T) {
	t.Parallel()

	report := benchmark.Report{
		Suite: benchmark.SuiteInfo{
			ID:           "firety.skill-lint-built-in",
			Name:         "Firety Built-in Skill Lint Benchmark",
			Version:      benchmark.SkillLintBenchmarkSuiteVersion,
			FixtureCount: 1,
		},
		Fixtures: []benchmark.FixtureResult{
			{
				Name:          "good-portable-skill",
				Intent:        "Well-authored portable skill with clear routing and examples.",
				Category:      benchmark.CategoryPortableQuality,
				CategoryLabel: benchmark.CategoryLabel(benchmark.CategoryPortableQuality),
				Passed:        true,
				Deterministic: true,
				ErrorCount:    0,
				WarningCount:  0,
				RoutingRisk:   "low",
				Summary:       "Benchmark expectations held.",
			},
		},
		Categories: []benchmark.CategorySummary{
			{
				Category:      benchmark.CategoryPortableQuality,
				CategoryLabel: benchmark.CategoryLabel(benchmark.CategoryPortableQuality),
				FixtureCount:  1,
				Passed:        1,
			},
		},
		Summary: benchmark.Summary{
			TotalFixtures:      1,
			PassedFixtures:     1,
			DeterministicCount: 1,
			StabilityOK:        true,
			ConfidenceSignals:  []string{"All built-in benchmark fixtures were deterministic across repeated lint runs."},
			Summary:            "All 1 built-in benchmark fixtures passed.",
		},
	}

	version := app.VersionInfo{
		Version: "test-version",
		Commit:  "abc1234",
		Date:    "2026-03-06T00:00:00Z",
	}

	left := artifact.BuildBenchmarkArtifact(version, report, artifact.BenchmarkArtifactOptions{Format: "json"}, 0)
	right := artifact.BuildBenchmarkArtifact(version, report, artifact.BenchmarkArtifactOptions{Format: "json"}, 0)

	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("marshal left artifact: %v", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("marshal right artifact: %v", err)
	}

	if string(leftJSON) != string(rightJSON) {
		t.Fatalf("expected deterministic benchmark artifact, left=%s right=%s", leftJSON, rightJSON)
	}
	if left.SchemaVersion != artifact.BenchmarkArtifactSchemaVersion {
		t.Fatalf("expected schema version %q, got %#v", artifact.BenchmarkArtifactSchemaVersion, left)
	}
	if left.ArtifactType != "firety.benchmark-report" {
		t.Fatalf("expected artifact type, got %#v", left)
	}
	if left.Fingerprint == "" {
		t.Fatalf("expected fingerprint, got %#v", left)
	}
}
