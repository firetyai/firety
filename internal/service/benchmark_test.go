package service_test

import (
	"testing"

	"github.com/firety/firety/internal/benchmark"
	"github.com/firety/firety/internal/service"
)

func TestBenchmarkServiceRunBuiltIn(t *testing.T) {
	t.Parallel()

	report, err := service.NewBenchmarkService(service.NewSkillLinter()).RunBuiltIn()
	if err != nil {
		t.Fatalf("run built-in benchmark: %v", err)
	}

	if report.Suite.ID != "firety.skill-lint-built-in" {
		t.Fatalf("expected benchmark suite id, got %#v", report.Suite)
	}
	if report.Summary.TotalFixtures != len(benchmark.SkillLintBenchmarkCorpus()) {
		t.Fatalf("expected all fixtures in summary, got %#v", report.Summary)
	}
	if report.Summary.FailedFixtures != 0 {
		t.Fatalf("expected current built-in benchmark to pass, got %#v", report.Summary)
	}
	if len(report.Categories) == 0 {
		t.Fatalf("expected category summaries, got %#v", report)
	}
	if !report.Summary.StabilityOK {
		t.Fatalf("expected stability signal, got %#v", report.Summary)
	}
}

func TestBenchmarkServiceRunDetectsRegressionAndNoise(t *testing.T) {
	t.Parallel()

	fixtures := []benchmark.BenchmarkSkillFixture{
		{
			Name:     "regressed-fixture",
			Intent:   "Fixture that should trigger explicit benchmark failures.",
			Category: benchmark.CategoryTriggerQuality,
			Files: map[string]string{
				"SKILL.md": "# Plain\n",
			},
			Expect: benchmark.BenchmarkExpectations{
				RequiredRuleIDs:  []string{"skill.generic-name"},
				ForbiddenRuleIDs: []string{"skill.short-content"},
				MaxWarningCount:  0,
				RoutingRiskLevel: "low",
			},
		},
	}

	report, err := service.NewBenchmarkService(service.NewSkillLinter()).Run(fixtures)
	if err != nil {
		t.Fatalf("run benchmark fixtures: %v", err)
	}

	if report.Summary.FailedFixtures != 1 {
		t.Fatalf("expected failing benchmark summary, got %#v", report.Summary)
	}
	if len(report.Summary.NotableRegressions) == 0 {
		t.Fatalf("expected notable regression summary, got %#v", report.Summary)
	}
	if len(report.Fixtures) != 1 || report.Fixtures[0].Passed {
		t.Fatalf("expected failing fixture result, got %#v", report.Fixtures)
	}
	if len(report.Fixtures[0].RegressionIssues) == 0 {
		t.Fatalf("expected regression issues, got %#v", report.Fixtures[0])
	}
	if len(report.Fixtures[0].NoiseIssues) == 0 {
		t.Fatalf("expected noisy finding issues, got %#v", report.Fixtures[0])
	}
}
