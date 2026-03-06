package lint

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

type ComparisonOutcome string
type RoutingRiskChange string

const (
	ComparisonImproved  ComparisonOutcome = "improved"
	ComparisonRegressed ComparisonOutcome = "regressed"
	ComparisonMixed     ComparisonOutcome = "mixed"
	ComparisonUnchanged ComparisonOutcome = "unchanged"

	RoutingRiskImproved  RoutingRiskChange = "improved"
	RoutingRiskRegressed RoutingRiskChange = "regressed"
	RoutingRiskUnchanged RoutingRiskChange = "unchanged"
)

type ComparisonSideSummary struct {
	Target       string `json:"target"`
	Valid        bool   `json:"valid"`
	ErrorCount   int    `json:"error_count"`
	WarningCount int    `json:"warning_count"`
	FindingCount int    `json:"finding_count"`
}

type ComparisonCountsDelta struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Findings int `json:"findings"`
}

type ComparisonFinding struct {
	RuleID   string   `json:"rule_id"`
	Category Category `json:"category,omitempty"`
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Line     *int     `json:"line,omitempty"`
	Message  string   `json:"message"`
}

type ComparisonChangedFinding struct {
	RuleID            string   `json:"rule_id"`
	Category          Category `json:"category,omitempty"`
	Path              string   `json:"path"`
	Message           string   `json:"message"`
	BaseSeverity      Severity `json:"base_severity"`
	CandidateSeverity Severity `json:"candidate_severity"`
	BaseLine          *int     `json:"base_line,omitempty"`
	CandidateLine     *int     `json:"candidate_line,omitempty"`
}

type ComparisonCategoryDelta struct {
	Category       Category `json:"category"`
	Title          string   `json:"title"`
	BaseCount      int      `json:"base_count"`
	CandidateCount int      `json:"candidate_count"`
	Delta          int      `json:"delta"`
}

type RoutingRiskDelta struct {
	Status                 RoutingRiskChange `json:"status"`
	BaseOverallRisk        RoutingRiskLevel  `json:"base_overall_routing_risk"`
	CandidateOverallRisk   RoutingRiskLevel  `json:"candidate_overall_routing_risk"`
	AddedRiskAreas         []string          `json:"added_risk_areas,omitempty"`
	RemovedRiskAreas       []string          `json:"removed_risk_areas,omitempty"`
	AddedPriorityActions   []string          `json:"added_priority_actions,omitempty"`
	RemovedPriorityActions []string          `json:"removed_priority_actions,omitempty"`
}

type ReportComparisonSummary struct {
	Overall                 ComparisonOutcome     `json:"overall"`
	Summary                 string                `json:"summary"`
	AddedCount              int                   `json:"added_count"`
	RemovedCount            int                   `json:"removed_count"`
	SeverityChangedCount    int                   `json:"severity_changed_count"`
	CountsDelta             ComparisonCountsDelta `json:"counts_delta"`
	HighPriorityRegressions []string              `json:"high_priority_regressions,omitempty"`
	NotableImprovements     []string              `json:"notable_improvements,omitempty"`
	RegressionAreas         []string              `json:"regression_areas,omitempty"`
	ImprovementAreas        []string              `json:"improvement_areas,omitempty"`
}

type ReportComparison struct {
	Base             ComparisonSideSummary      `json:"base"`
	Candidate        ComparisonSideSummary      `json:"candidate"`
	Summary          ReportComparisonSummary    `json:"summary"`
	AddedFindings    []ComparisonFinding        `json:"added_findings,omitempty"`
	RemovedFindings  []ComparisonFinding        `json:"removed_findings,omitempty"`
	ChangedFindings  []ComparisonChangedFinding `json:"changed_findings,omitempty"`
	CategoryDeltas   []ComparisonCategoryDelta  `json:"category_deltas,omitempty"`
	RoutingRiskDelta RoutingRiskDelta           `json:"routing_risk_delta"`
}

type findingOccurrence struct {
	ruleID   string
	category Category
	severity Severity
	path     string
	line     int
	message  string
}

func CompareReports(base, candidate Report) ReportComparison {
	added, removed, changed := compareFindings(base.Findings, candidate.Findings)
	categoryDeltas := compareCategories(base.Findings, candidate.Findings)
	routingDelta := compareRoutingRisk(base.Findings, candidate.Findings)

	summary := ReportComparisonSummary{
		AddedCount:           len(added),
		RemovedCount:         len(removed),
		SeverityChangedCount: len(changed),
		CountsDelta: ComparisonCountsDelta{
			Errors:   candidate.ErrorCount() - base.ErrorCount(),
			Warnings: candidate.WarningCount() - base.WarningCount(),
			Findings: len(candidate.Findings) - len(base.Findings),
		},
	}

	hasRegression := len(added) > 0 || len(changed) > 0 || routingDelta.Status == RoutingRiskRegressed
	hasImprovement := len(removed) > 0 || routingDelta.Status == RoutingRiskImproved
	switch {
	case hasRegression && hasImprovement:
		summary.Overall = ComparisonMixed
		summary.Summary = "The candidate resolves some issues but also introduces new lint risk."
	case hasRegression:
		summary.Overall = ComparisonRegressed
		summary.Summary = "The candidate introduces lint regressions or higher routing risk."
	case hasImprovement:
		summary.Overall = ComparisonImproved
		summary.Summary = "The candidate resolves lint issues without introducing new regressions."
	default:
		summary.Overall = ComparisonUnchanged
		summary.Summary = "Firety did not detect meaningful lint-quality changes between the two versions."
	}

	summary.RegressionAreas, summary.ImprovementAreas = summarizeComparisonAreas(categoryDeltas, routingDelta)
	summary.HighPriorityRegressions = summarizeHighPriorityRegressions(added, changed, routingDelta)
	summary.NotableImprovements = summarizeNotableImprovements(removed, routingDelta)

	return ReportComparison{
		Base: ComparisonSideSummary{
			Target:       base.Target,
			Valid:        !base.HasErrors(),
			ErrorCount:   base.ErrorCount(),
			WarningCount: base.WarningCount(),
			FindingCount: len(base.Findings),
		},
		Candidate: ComparisonSideSummary{
			Target:       candidate.Target,
			Valid:        !candidate.HasErrors(),
			ErrorCount:   candidate.ErrorCount(),
			WarningCount: candidate.WarningCount(),
			FindingCount: len(candidate.Findings),
		},
		Summary:          summary,
		AddedFindings:    added,
		RemovedFindings:  removed,
		ChangedFindings:  changed,
		CategoryDeltas:   categoryDeltas,
		RoutingRiskDelta: routingDelta,
	}
}

func compareFindings(base, candidate []Finding) ([]ComparisonFinding, []ComparisonFinding, []ComparisonChangedFinding) {
	baseGroups := groupFindings(base)
	candidateGroups := groupFindings(candidate)

	keys := make([]string, 0, len(baseGroups)+len(candidateGroups))
	seen := make(map[string]struct{}, len(baseGroups)+len(candidateGroups))
	for key := range baseGroups {
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	for key := range candidateGroups {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	added := make([]ComparisonFinding, 0)
	removed := make([]ComparisonFinding, 0)
	changed := make([]ComparisonChangedFinding, 0)

	for _, key := range keys {
		baseOccurrences := slices.Clone(baseGroups[key])
		candidateOccurrences := slices.Clone(candidateGroups[key])

		pairedCount := min(len(baseOccurrences), len(candidateOccurrences))
		for index := 0; index < pairedCount; index++ {
			baseOccurrence := baseOccurrences[index]
			candidateOccurrence := candidateOccurrences[index]
			if baseOccurrence.severity == candidateOccurrence.severity {
				continue
			}

			changed = append(changed, ComparisonChangedFinding{
				RuleID:            candidateOccurrence.ruleID,
				Category:          candidateOccurrence.category,
				Path:              candidateOccurrence.path,
				Message:           candidateOccurrence.message,
				BaseSeverity:      baseOccurrence.severity,
				CandidateSeverity: candidateOccurrence.severity,
				BaseLine:          linePointer(baseOccurrence.line),
				CandidateLine:     linePointer(candidateOccurrence.line),
			})
		}

		for _, occurrence := range candidateOccurrences[pairedCount:] {
			added = append(added, comparisonFindingFromOccurrence(occurrence))
		}
		for _, occurrence := range baseOccurrences[pairedCount:] {
			removed = append(removed, comparisonFindingFromOccurrence(occurrence))
		}
	}

	sort.SliceStable(added, func(i, j int) bool {
		return comparisonFindingLess(added[i], added[j])
	})
	sort.SliceStable(removed, func(i, j int) bool {
		return comparisonFindingLess(removed[i], removed[j])
	})
	sort.SliceStable(changed, func(i, j int) bool {
		if severityRank(changed[i].CandidateSeverity) != severityRank(changed[j].CandidateSeverity) {
			return severityRank(changed[i].CandidateSeverity) < severityRank(changed[j].CandidateSeverity)
		}
		if changed[i].RuleID != changed[j].RuleID {
			return changed[i].RuleID < changed[j].RuleID
		}
		if changed[i].Path != changed[j].Path {
			return changed[i].Path < changed[j].Path
		}
		return changed[i].Message < changed[j].Message
	})

	return added, removed, changed
}

func groupFindings(findings []Finding) map[string][]findingOccurrence {
	grouped := make(map[string][]findingOccurrence, len(findings))

	for _, finding := range findings {
		category := Category("")
		if rule, ok := FindRule(finding.RuleID); ok {
			category = rule.Category
		}

		occurrence := findingOccurrence{
			ruleID:   finding.RuleID,
			category: category,
			severity: finding.Severity,
			path:     finding.Path,
			line:     finding.Line,
			message:  finding.Message,
		}
		key := findingComparisonKey(occurrence)
		grouped[key] = append(grouped[key], occurrence)
	}

	for key := range grouped {
		sort.SliceStable(grouped[key], func(i, j int) bool {
			if severityRank(grouped[key][i].severity) != severityRank(grouped[key][j].severity) {
				return severityRank(grouped[key][i].severity) < severityRank(grouped[key][j].severity)
			}
			return grouped[key][i].line < grouped[key][j].line
		})
	}

	return grouped
}

func findingComparisonKey(occurrence findingOccurrence) string {
	return strings.Join([]string{occurrence.ruleID, occurrence.path, occurrence.message}, "\x00")
}

func comparisonFindingFromOccurrence(occurrence findingOccurrence) ComparisonFinding {
	return ComparisonFinding{
		RuleID:   occurrence.ruleID,
		Category: occurrence.category,
		Severity: occurrence.severity,
		Path:     occurrence.path,
		Line:     linePointer(occurrence.line),
		Message:  occurrence.message,
	}
}

func comparisonFindingLess(left, right ComparisonFinding) bool {
	if severityRank(left.Severity) != severityRank(right.Severity) {
		return severityRank(left.Severity) < severityRank(right.Severity)
	}
	if left.RuleID != right.RuleID {
		return left.RuleID < right.RuleID
	}
	if left.Path != right.Path {
		return left.Path < right.Path
	}
	return left.Message < right.Message
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityError:
		return 0
	default:
		return 1
	}
}

func linePointer(line int) *int {
	if line <= 0 {
		return nil
	}

	lineCopy := line
	return &lineCopy
}

func compareCategories(base, candidate []Finding) []ComparisonCategoryDelta {
	baseCounts := countCategories(base)
	candidateCounts := countCategories(candidate)
	deltas := make([]ComparisonCategoryDelta, 0, len(allComparisonCategories()))

	for _, category := range allComparisonCategories() {
		baseCount := baseCounts[category]
		candidateCount := candidateCounts[category]
		delta := candidateCount - baseCount
		if delta == 0 {
			continue
		}

		deltas = append(deltas, ComparisonCategoryDelta{
			Category:       category,
			Title:          comparisonCategoryTitle(category),
			BaseCount:      baseCount,
			CandidateCount: candidateCount,
			Delta:          delta,
		})
	}

	sort.SliceStable(deltas, func(i, j int) bool {
		leftAbs := absInt(deltas[i].Delta)
		rightAbs := absInt(deltas[j].Delta)
		if leftAbs != rightAbs {
			return leftAbs > rightAbs
		}
		return deltas[i].Category < deltas[j].Category
	})

	return deltas
}

func countCategories(findings []Finding) map[Category]int {
	counts := make(map[Category]int)
	for _, finding := range findings {
		rule, ok := FindRule(finding.RuleID)
		if !ok {
			continue
		}
		counts[rule.Category]++
	}
	return counts
}

func allComparisonCategories() []Category {
	return []Category{
		CategoryStructure,
		CategoryMetadataSpec,
		CategoryInvocation,
		CategoryExamples,
		CategoryNegativeGuidance,
		CategoryConsistency,
		CategoryPortability,
		CategoryBundleResources,
		CategoryEfficiencyCost,
		CategoryTriggerQuality,
	}
}

func comparisonCategoryTitle(category Category) string {
	switch category {
	case CategoryStructure:
		return "Structure"
	case CategoryMetadataSpec:
		return "Metadata and spec"
	case CategoryInvocation:
		return "Invocation guidance"
	case CategoryExamples:
		return "Examples"
	case CategoryNegativeGuidance:
		return "Negative guidance"
	case CategoryConsistency:
		return "Consistency"
	case CategoryPortability:
		return "Portability"
	case CategoryBundleResources:
		return "Bundle resources"
	case CategoryEfficiencyCost:
		return "Efficiency and cost"
	case CategoryTriggerQuality:
		return "Trigger quality"
	default:
		return string(category)
	}
}

func compareRoutingRisk(base, candidate []Finding) RoutingRiskDelta {
	baseSummary := SummarizeRoutingRisk(base)
	candidateSummary := SummarizeRoutingRisk(candidate)

	status := RoutingRiskUnchanged
	baseRank := routingRiskRank(baseSummary.OverallRisk)
	candidateRank := routingRiskRank(candidateSummary.OverallRisk)
	switch {
	case candidateRank > baseRank:
		status = RoutingRiskRegressed
	case candidateRank < baseRank:
		status = RoutingRiskImproved
	}

	return RoutingRiskDelta{
		Status:                 status,
		BaseOverallRisk:        baseSummary.OverallRisk,
		CandidateOverallRisk:   candidateSummary.OverallRisk,
		AddedRiskAreas:         addedStrings(routingRiskAreaKeys(candidateSummary.RiskAreas), routingRiskAreaKeys(baseSummary.RiskAreas)),
		RemovedRiskAreas:       addedStrings(routingRiskAreaKeys(baseSummary.RiskAreas), routingRiskAreaKeys(candidateSummary.RiskAreas)),
		AddedPriorityActions:   addedStrings(candidateSummary.PriorityActions, baseSummary.PriorityActions),
		RemovedPriorityActions: addedStrings(baseSummary.PriorityActions, candidateSummary.PriorityActions),
	}
}

func routingRiskAreaKeys(areas []RoutingRiskArea) []string {
	keys := make([]string, 0, len(areas))
	for _, area := range areas {
		keys = append(keys, area.Key)
	}
	return keys
}

func addedStrings(values, baseline []string) []string {
	seen := make(map[string]struct{}, len(baseline))
	for _, value := range baseline {
		seen[value] = struct{}{}
	}

	added := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		added = append(added, value)
	}

	return added
}

func routingRiskRank(level RoutingRiskLevel) int {
	switch level {
	case RoutingRiskHigh:
		return 3
	case RoutingRiskMedium:
		return 2
	default:
		return 1
	}
}

func summarizeComparisonAreas(deltas []ComparisonCategoryDelta, routingDelta RoutingRiskDelta) ([]string, []string) {
	regressions := make([]string, 0, 4)
	improvements := make([]string, 0, 4)

	for _, delta := range deltas {
		switch {
		case delta.Delta > 0:
			regressions = append(regressions, fmt.Sprintf("%s (+%d finding%s)", delta.Title, delta.Delta, pluralSuffix(delta.Delta)))
		case delta.Delta < 0:
			improvements = append(improvements, fmt.Sprintf("%s (%d finding%s)", delta.Title, delta.Delta, pluralSuffix(absInt(delta.Delta))))
		}
	}

	if routingDelta.Status == RoutingRiskRegressed {
		regressions = append(regressions, fmt.Sprintf("Routing risk (%s to %s)", routingDelta.BaseOverallRisk, routingDelta.CandidateOverallRisk))
	}
	if routingDelta.Status == RoutingRiskImproved {
		improvements = append(improvements, fmt.Sprintf("Routing risk (%s to %s)", routingDelta.BaseOverallRisk, routingDelta.CandidateOverallRisk))
	}

	if len(regressions) > 3 {
		regressions = regressions[:3]
	}
	if len(improvements) > 3 {
		improvements = improvements[:3]
	}

	return regressions, improvements
}

func summarizeHighPriorityRegressions(added []ComparisonFinding, changed []ComparisonChangedFinding, routingDelta RoutingRiskDelta) []string {
	summaries := make([]string, 0, 3)

	errorCount := 0
	for _, finding := range added {
		if finding.Severity == SeverityError {
			errorCount++
		}
	}
	if errorCount > 0 {
		summaries = append(summaries, fmt.Sprintf("%d new error%s were introduced.", errorCount, pluralSuffix(errorCount)))
	}

	escalationCount := 0
	for _, finding := range changed {
		if finding.CandidateSeverity == SeverityError && finding.BaseSeverity != SeverityError {
			escalationCount++
		}
	}
	if escalationCount > 0 {
		summaries = append(summaries, fmt.Sprintf("%d existing finding%s escalated to errors.", escalationCount, pluralSuffix(escalationCount)))
	}

	if routingDelta.Status == RoutingRiskRegressed {
		summaries = append(summaries, fmt.Sprintf("Routing risk regressed from %s to %s.", routingDelta.BaseOverallRisk, routingDelta.CandidateOverallRisk))
	}

	if len(summaries) > 3 {
		return summaries[:3]
	}
	return summaries
}

func summarizeNotableImprovements(removed []ComparisonFinding, routingDelta RoutingRiskDelta) []string {
	summaries := make([]string, 0, 3)

	errorCount := 0
	for _, finding := range removed {
		if finding.Severity == SeverityError {
			errorCount++
		}
	}
	if errorCount > 0 {
		summaries = append(summaries, fmt.Sprintf("%d error%s were resolved.", errorCount, pluralSuffix(errorCount)))
	}

	warningCount := 0
	for _, finding := range removed {
		if finding.Severity == SeverityWarning {
			warningCount++
		}
	}
	if warningCount > 0 {
		summaries = append(summaries, fmt.Sprintf("%d warning%s were resolved.", warningCount, pluralSuffix(warningCount)))
	}

	if routingDelta.Status == RoutingRiskImproved {
		summaries = append(summaries, fmt.Sprintf("Routing risk improved from %s to %s.", routingDelta.BaseOverallRisk, routingDelta.CandidateOverallRisk))
	}

	if len(summaries) > 3 {
		return summaries[:3]
	}
	return summaries
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
