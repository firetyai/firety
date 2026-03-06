package service_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/firety/firety/internal/testutil"
)

func TestSkillLintBenchmarkCorpusCoverage(t *testing.T) {
	t.Parallel()

	fixtures := testutil.SkillLintBenchmarkCorpus()
	expectedNames := []string{
		"good-portable-skill",
		"structurally-broken-skill",
		"vague-generic-skill",
		"diffuse-overbroad-skill",
		"mixed-ecosystem-portability",
		"bundle-resource-problem",
		"cost-bloat-problem",
		"example-quality-problem",
		"good-intentional-codex-skill",
		"accidentally-tool-locked-skill",
	}

	if len(fixtures) != len(expectedNames) {
		t.Fatalf("expected %d benchmark fixtures, got %d", len(expectedNames), len(fixtures))
	}

	for index, expected := range expectedNames {
		if fixtures[index].Name != expected {
			t.Fatalf("expected fixture %d to be %q, got %#v", index, expected, fixtures[index])
		}
		if fixtures[index].Intent == "" {
			t.Fatalf("expected fixture %q to have intent metadata", fixtures[index].Name)
		}
	}
}

func TestSkillLintBenchmarkCorpusRegression(t *testing.T) {
	t.Parallel()

	version := app.VersionInfo{
		Version: "test-version",
		Commit:  "abc1234",
		Date:    "2026-03-06T00:00:00Z",
	}

	for _, fixture := range testutil.SkillLintBenchmarkCorpus() {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			testutil.WriteFiles(t, root, fixture.Files)

			profile := service.SkillLintProfileGeneric
			if fixture.Profile != "" {
				parsedProfile, err := service.ParseSkillLintProfile(fixture.Profile)
				if err != nil {
					t.Fatalf("parse profile: %v", err)
				}
				profile = parsedProfile
			}

			strictness := lint.StrictnessDefault
			if fixture.Strictness != "" {
				parsedStrictness, err := lint.ParseStrictness(fixture.Strictness)
				if err != nil {
					t.Fatalf("parse strictness: %v", err)
				}
				strictness = parsedStrictness
			}

			linter := service.NewSkillLinter()
			report, err := linter.LintWithProfileAndStrictness(root, profile, strictness)
			if err != nil {
				t.Fatalf("lint fixture: %v", err)
			}

			repeatedReport, err := linter.LintWithProfileAndStrictness(root, profile, strictness)
			if err != nil {
				t.Fatalf("rerun lint fixture: %v", err)
			}
			if !reflect.DeepEqual(report.Findings, repeatedReport.Findings) {
				t.Fatalf("expected deterministic findings, first=%#v second=%#v", report.Findings, repeatedReport.Findings)
			}

			expect := fixture.Expect
			if expect.MaxErrorCount == 0 && expect.MinErrorCount > 0 {
				expect.MaxErrorCount = -1
			}
			if expect.MaxWarningCount == 0 && expect.MinWarningCount > 0 {
				expect.MaxWarningCount = -1
			}

			ruleIDs := findingRuleIDs(report.Findings)
			for _, required := range expect.RequiredRuleIDs {
				if !slices.Contains(ruleIDs, required) {
					t.Fatalf("expected rule %q in findings, got %#v", required, ruleIDs)
				}
			}

			for _, forbidden := range expect.ForbiddenRuleIDs {
				if slices.Contains(ruleIDs, forbidden) {
					t.Fatalf("expected rule %q to stay absent, got %#v", forbidden, ruleIDs)
				}
			}

			if report.ErrorCount() < expect.MinErrorCount {
				t.Fatalf("expected at least %d error(s), got %#v", expect.MinErrorCount, report.Findings)
			}
			if expect.MaxErrorCount >= 0 && report.ErrorCount() > expect.MaxErrorCount {
				t.Fatalf("expected at most %d error(s), got %#v", expect.MaxErrorCount, report.Findings)
			}
			if report.WarningCount() < expect.MinWarningCount {
				t.Fatalf("expected at least %d warning(s), got %#v", expect.MinWarningCount, report.Findings)
			}
			if expect.MaxWarningCount >= 0 && report.WarningCount() > expect.MaxWarningCount {
				t.Fatalf("expected at most %d warning(s), got %#v", expect.MaxWarningCount, report.Findings)
			}

			routingRisk := lint.SummarizeRoutingRisk(report.Findings)
			if expect.RoutingRiskLevel != "" && routingRisk.OverallRisk != expect.RoutingRiskLevel {
				t.Fatalf("expected routing risk %q, got %#v", expect.RoutingRiskLevel, routingRisk)
			}
			if len(expect.RoutingRiskAreas) > 0 {
				areaKeys := routingRiskAreaKeys(routingRisk.RiskAreas)
				for _, expectedArea := range expect.RoutingRiskAreas {
					if !slices.Contains(areaKeys, expectedArea) {
						t.Fatalf("expected routing risk area %q, got %#v", expectedArea, areaKeys)
					}
				}
			}

			if expect.VerifyArtifact {
				exitCode := 0
				if report.HasErrors() {
					exitCode = 1
				}
				lintArtifact := artifact.BuildSkillLintArtifact(version, report, service.SkillFixResult{}, artifact.SkillLintArtifactOptions{
					Format:      "json",
					Profile:     string(profile),
					Strictness:  string(strictness),
					FailOn:      "errors",
					Explain:     true,
					RoutingRisk: true,
				}, exitCode)

				if lintArtifact.SchemaVersion != artifact.SkillLintArtifactSchemaVersion {
					t.Fatalf("expected schema version %q, got %#v", artifact.SkillLintArtifactSchemaVersion, lintArtifact)
				}
				if lintArtifact.RoutingRisk == nil {
					t.Fatalf("expected routing risk in artifact, got %#v", lintArtifact)
				}
				for _, required := range expect.RequiredRuleIDs {
					if !artifactHasRule(lintArtifact, required) {
						t.Fatalf("expected artifact to reference rule %q, got %#v", required, lintArtifact.RuleCatalog)
					}
				}
			}
		})
	}
}

func findingRuleIDs(findings []lint.Finding) []string {
	ruleIDs := make([]string, 0, len(findings))
	for _, finding := range findings {
		ruleIDs = append(ruleIDs, finding.RuleID)
	}
	return ruleIDs
}

func routingRiskAreaKeys(areas []lint.RoutingRiskArea) []string {
	keys := make([]string, 0, len(areas))
	for _, area := range areas {
		keys = append(keys, area.Key)
	}
	return keys
}

func artifactHasRule(artifact artifact.SkillLintArtifact, ruleID string) bool {
	for _, rule := range artifact.RuleCatalog {
		if rule.ID == ruleID {
			return true
		}
	}
	return false
}
