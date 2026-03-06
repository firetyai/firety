package service

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/firety/firety/internal/benchmark"
	"github.com/firety/firety/internal/domain/lint"
)

type BenchmarkService struct {
	linter SkillLinter
}

func NewBenchmarkService(linter SkillLinter) BenchmarkService {
	return BenchmarkService{linter: linter}
}

func (s BenchmarkService) RunBuiltIn() (benchmark.Report, error) {
	return s.Run(benchmark.SkillLintBenchmarkCorpus())
}

func (s BenchmarkService) Run(fixtures []benchmark.BenchmarkSkillFixture) (benchmark.Report, error) {
	results := make([]benchmark.FixtureResult, 0, len(fixtures))

	for _, fixture := range fixtures {
		result, err := s.runFixture(fixture)
		if err != nil {
			return benchmark.Report{}, fmt.Errorf("benchmark fixture %q: %w", fixture.Name, err)
		}
		results = append(results, result)
	}

	return benchmark.Report{
		Suite: benchmark.SuiteInfo{
			ID:           "firety.skill-lint-built-in",
			Name:         "Firety Built-in Skill Lint Benchmark",
			Version:      benchmark.SkillLintBenchmarkSuiteVersion,
			FixtureCount: len(results),
		},
		Fixtures:   results,
		Categories: summarizeBenchmarkCategories(results),
		Summary:    summarizeBenchmarkReport(results),
	}, nil
}

func (s BenchmarkService) runFixture(fixture benchmark.BenchmarkSkillFixture) (benchmark.FixtureResult, error) {
	root, err := os.MkdirTemp("", "firety-benchmark-*")
	if err != nil {
		return benchmark.FixtureResult{}, err
	}
	defer os.RemoveAll(root)

	if err := writeBenchmarkFixtureFiles(root, fixture.Files); err != nil {
		return benchmark.FixtureResult{}, err
	}

	profile := SkillLintProfileGeneric
	if fixture.Profile != "" {
		profile, err = ParseSkillLintProfile(fixture.Profile)
		if err != nil {
			return benchmark.FixtureResult{}, err
		}
	}

	strictness := lint.StrictnessDefault
	if fixture.Strictness != "" {
		strictness, err = lint.ParseStrictness(fixture.Strictness)
		if err != nil {
			return benchmark.FixtureResult{}, err
		}
	}

	report, err := s.linter.LintWithProfileAndStrictness(root, profile, strictness)
	if err != nil {
		return benchmark.FixtureResult{}, err
	}
	repeatedReport, err := s.linter.LintWithProfileAndStrictness(root, profile, strictness)
	if err != nil {
		return benchmark.FixtureResult{}, err
	}

	deterministic := slices.EqualFunc(report.Findings, repeatedReport.Findings, func(left, right lint.Finding) bool {
		return left == right
	})
	routingRisk := lint.SummarizeRoutingRisk(report.Findings)
	routingRiskAreas := make([]string, 0, len(routingRisk.RiskAreas))
	for _, area := range routingRisk.RiskAreas {
		routingRiskAreas = append(routingRiskAreas, area.Key)
	}

	ruleIDs := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		ruleIDs = append(ruleIDs, finding.RuleID)
	}

	expect := fixture.Expect
	if expect.MaxErrorCount == 0 && expect.MinErrorCount > 0 {
		expect.MaxErrorCount = -1
	}
	if expect.MaxWarningCount == 0 && expect.MinWarningCount > 0 {
		expect.MaxWarningCount = -1
	}
	missingRequired := difference(expect.RequiredRuleIDs, ruleIDs)
	unexpectedRules := intersection(expect.ForbiddenRuleIDs, ruleIDs)
	regressionIssues := make([]string, 0)
	noiseIssues := make([]string, 0)

	for _, ruleID := range missingRequired {
		regressionIssues = append(regressionIssues, fmt.Sprintf("missing expected rule %s", ruleID))
	}
	for _, ruleID := range unexpectedRules {
		noiseIssues = append(noiseIssues, fmt.Sprintf("unexpected noisy rule %s", ruleID))
	}

	if report.ErrorCount() < expect.MinErrorCount {
		regressionIssues = append(regressionIssues, fmt.Sprintf("expected at least %d error(s), got %d", expect.MinErrorCount, report.ErrorCount()))
	}
	if expect.MaxErrorCount >= 0 && report.ErrorCount() > expect.MaxErrorCount {
		noiseIssues = append(noiseIssues, fmt.Sprintf("expected at most %d error(s), got %d", expect.MaxErrorCount, report.ErrorCount()))
	}
	if report.WarningCount() < expect.MinWarningCount {
		regressionIssues = append(regressionIssues, fmt.Sprintf("expected at least %d warning(s), got %d", expect.MinWarningCount, report.WarningCount()))
	}
	if expect.MaxWarningCount >= 0 && report.WarningCount() > expect.MaxWarningCount {
		noiseIssues = append(noiseIssues, fmt.Sprintf("expected at most %d warning(s), got %d", expect.MaxWarningCount, report.WarningCount()))
	}
	if expect.RoutingRiskLevel != "" && routingRisk.OverallRisk != expect.RoutingRiskLevel {
		regressionIssues = append(regressionIssues, fmt.Sprintf("expected routing risk %s, got %s", expect.RoutingRiskLevel, routingRisk.OverallRisk))
	}
	for _, area := range expect.RoutingRiskAreas {
		if !slices.Contains(routingRiskAreas, area) {
			regressionIssues = append(regressionIssues, fmt.Sprintf("missing routing risk area %s", area))
		}
	}
	if !deterministic {
		regressionIssues = append(regressionIssues, "findings changed across repeated benchmark runs")
	}

	passed := len(regressionIssues) == 0 && len(noiseIssues) == 0
	summary := "Benchmark expectations held."
	switch {
	case len(regressionIssues) > 0 && len(noiseIssues) > 0:
		summary = fmt.Sprintf("%s; %s.", regressionIssues[0], noiseIssues[0])
	case len(regressionIssues) > 0:
		summary = regressionIssues[0] + "."
	case len(noiseIssues) > 0:
		summary = noiseIssues[0] + "."
	}

	return benchmark.FixtureResult{
		Name:                   fixture.Name,
		Intent:                 fixture.Intent,
		Category:               fixture.Category,
		CategoryLabel:          benchmark.CategoryLabel(fixture.Category),
		Profile:                fixture.Profile,
		Strictness:             fixture.Strictness,
		Passed:                 passed,
		Deterministic:          deterministic,
		ErrorCount:             report.ErrorCount(),
		WarningCount:           report.WarningCount(),
		RoutingRisk:            string(routingRisk.OverallRisk),
		RoutingRiskAreas:       routingRiskAreas,
		MissingRequiredRuleIDs: missingRequired,
		UnexpectedRuleIDs:      unexpectedRules,
		RegressionIssues:       regressionIssues,
		NoiseIssues:            noiseIssues,
		Summary:                summary,
	}, nil
}

func summarizeBenchmarkCategories(results []benchmark.FixtureResult) []benchmark.CategorySummary {
	categories := make([]benchmark.CategorySummary, 0)
	index := make(map[benchmark.FixtureCategory]int)

	for _, result := range results {
		position, ok := index[result.Category]
		if !ok {
			position = len(categories)
			index[result.Category] = position
			categories = append(categories, benchmark.CategorySummary{
				Category:      result.Category,
				CategoryLabel: result.CategoryLabel,
			})
		}

		entry := &categories[position]
		entry.FixtureCount++
		if result.Passed {
			entry.Passed++
		} else {
			entry.Failed++
		}
	}

	return categories
}

func summarizeBenchmarkReport(results []benchmark.FixtureResult) benchmark.Summary {
	summary := benchmark.Summary{
		TotalFixtures: len(results),
		StabilityOK:   true,
	}

	for _, result := range results {
		if result.Passed {
			summary.PassedFixtures++
		} else {
			summary.FailedFixtures++
		}
		if result.Deterministic {
			summary.DeterministicCount++
		} else {
			summary.StabilityOK = false
		}
		if !result.Passed && len(summary.NotableRegressions) < 5 {
			summary.NotableRegressions = append(summary.NotableRegressions, fmt.Sprintf("%s: %s", result.Name, result.Summary))
		}
		if len(result.NoiseIssues) > 0 && len(summary.NotableNoise) < 5 {
			summary.NotableNoise = append(summary.NotableNoise, fmt.Sprintf("%s: %s", result.Name, result.NoiseIssues[0]))
		}
	}

	if summary.DeterministicCount == summary.TotalFixtures {
		summary.ConfidenceSignals = append(summary.ConfidenceSignals, "All built-in benchmark fixtures were deterministic across repeated lint runs.")
	}
	if summary.FailedFixtures == 0 {
		summary.ConfidenceSignals = append(summary.ConfidenceSignals, "All benchmark invariants held for the current Firety build.")
	}
	if summary.TotalFixtures > 0 && len(summary.NotableNoise) == 0 {
		summary.ConfidenceSignals = append(summary.ConfidenceSignals, "Good and intentionally targeted fixtures stayed low-noise under the current rule set.")
	}

	switch {
	case summary.TotalFixtures == 0:
		summary.Summary = "No benchmark fixtures were available."
	case summary.FailedFixtures == 0:
		summary.Summary = fmt.Sprintf("All %d built-in benchmark fixtures passed.", summary.TotalFixtures)
	default:
		summary.Summary = fmt.Sprintf("%d of %d built-in benchmark fixtures failed.", summary.FailedFixtures, summary.TotalFixtures)
	}

	return summary
}

func writeBenchmarkFixtureFiles(root string, files map[string]string) error {
	for path, content := range files {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func difference(expected, actual []string) []string {
	result := make([]string, 0)
	for _, item := range expected {
		if !slices.Contains(actual, item) {
			result = append(result, item)
		}
	}
	return result
}

func intersection(expected, actual []string) []string {
	result := make([]string, 0)
	for _, item := range expected {
		if slices.Contains(actual, item) {
			result = append(result, item)
		}
	}
	return result
}

func BenchmarkSummaryStatus(report benchmark.Report) string {
	if report.Summary.FailedFixtures == 0 {
		return "healthy"
	}
	if report.Summary.PassedFixtures == 0 {
		return "regressed"
	}
	return "attention needed"
}

func BenchmarkReviewFirst(report benchmark.Report, limit int) []string {
	items := make([]string, 0, limit)
	for _, fixture := range report.Fixtures {
		if fixture.Passed {
			continue
		}
		items = append(items, fmt.Sprintf("%s: %s", fixture.Name, fixture.Summary))
		if len(items) == limit {
			break
		}
	}
	return items
}

func BenchmarkCategoryOverview(summary benchmark.CategorySummary) string {
	return fmt.Sprintf("%s: %d/%d passed", summary.CategoryLabel, summary.Passed, summary.FixtureCount)
}

func BenchmarkConfidenceSummary(report benchmark.Report) string {
	if len(report.Summary.ConfidenceSignals) == 0 {
		return ""
	}
	return strings.Join(report.Summary.ConfidenceSignals, " ")
}
