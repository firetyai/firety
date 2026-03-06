package compatibility

import (
	"fmt"
	"slices"
	"strings"

	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type SupportPosture string
type EvidenceLevel string
type Status string

const (
	SupportPostureGenericPortable         SupportPosture = "generic-portable"
	SupportPostureIntentionalToolSpecific SupportPosture = "intentionally-tool-specific"
	SupportPostureMixedAmbiguous          SupportPosture = "mixed-ambiguous"
	SupportPostureAccidentalToolLocked    SupportPosture = "accidentally-tool-locked"
	SupportPostureWeakEvidence            SupportPosture = "weak-evidence"

	EvidenceLevelStrong  EvidenceLevel = "strong"
	EvidenceLevelPartial EvidenceLevel = "partial"
	EvidenceLevelLimited EvidenceLevel = "limited"

	StatusStrong  Status = "strong"
	StatusMixed   Status = "mixed"
	StatusRisky   Status = "risky"
	StatusUnknown Status = "unknown"
)

type Report struct {
	Target                 string           `json:"target"`
	SupportPosture         SupportPosture   `json:"support_posture"`
	EvidenceLevel          EvidenceLevel    `json:"evidence_level"`
	Summary                string           `json:"summary"`
	Profiles               []ProfileSummary `json:"profiles,omitempty"`
	Backends               []BackendSummary `json:"backends,omitempty"`
	Blockers               []string         `json:"blockers,omitempty"`
	Strengths              []string         `json:"strengths,omitempty"`
	RecommendedPositioning string           `json:"recommended_positioning,omitempty"`
	PortabilityRiskSummary string           `json:"portability_risk_summary,omitempty"`
	RoutingRiskContext     string           `json:"routing_risk_context,omitempty"`
}

type ProfileSummary struct {
	Profile                 string                `json:"profile"`
	DisplayName             string                `json:"display_name"`
	Status                  Status                `json:"status"`
	Summary                 string                `json:"summary"`
	ErrorCount              int                   `json:"error_count"`
	WarningCount            int                   `json:"warning_count"`
	RoutingRisk             lint.RoutingRiskLevel `json:"routing_risk"`
	TargetingPosture        lint.TargetingPosture `json:"targeting_posture,omitempty"`
	PortabilityFindingCount int                   `json:"portability_finding_count"`
	RuleIDs                 []string              `json:"rule_ids,omitempty"`
}

type BackendSummary struct {
	BackendID          string  `json:"backend_id"`
	BackendName        string  `json:"backend_name"`
	Status             Status  `json:"status"`
	Summary            string  `json:"summary"`
	PassRate           float64 `json:"pass_rate"`
	FalsePositives     int     `json:"false_positives"`
	FalseNegatives     int     `json:"false_negatives"`
	Failed             int     `json:"failed"`
	DifferingCaseCount int     `json:"differing_case_count,omitempty"`
}

type Evidence struct {
	Target   string
	Profiles []ProfileSummary
	Backends []BackendSummary
}

func BuildReport(evidence Evidence) Report {
	profiles := append([]ProfileSummary(nil), evidence.Profiles...)
	backends := append([]BackendSummary(nil), evidence.Backends...)
	sortProfiles(profiles)
	sortBackends(backends)

	posture := classifySupportPosture(profiles, backends)
	level := classifyEvidenceLevel(profiles, backends)
	blockers := summarizeBlockers(posture, profiles, backends)
	strengths := summarizeStrengths(posture, profiles, backends)

	report := Report{
		Target:                 evidence.Target,
		SupportPosture:         posture,
		EvidenceLevel:          level,
		Summary:                summarizeReport(posture, level, profiles, backends),
		Profiles:               profiles,
		Backends:               backends,
		Blockers:               blockers,
		Strengths:              strengths,
		RecommendedPositioning: recommendedPositioning(posture, profiles, backends),
		PortabilityRiskSummary: portabilityRiskSummary(profiles),
		RoutingRiskContext:     routingRiskContext(profiles),
	}
	return report
}

func ProfileSummaryFromLint(profile string, report lint.Report) ProfileSummary {
	risk := lint.SummarizeRoutingRisk(report.Findings)
	ctx := lint.NewExplainContext(report.Findings, profile, lint.StrictnessDefault)
	portabilityCount := 0
	ruleIDs := make([]string, 0)
	for _, finding := range report.Findings {
		rule, ok := lint.FindRule(finding.RuleID)
		if !ok {
			continue
		}
		if rule.Category == lint.CategoryPortability {
			portabilityCount++
		}
		if !slices.Contains(ruleIDs, finding.RuleID) {
			ruleIDs = append(ruleIDs, finding.RuleID)
		}
	}
	slices.Sort(ruleIDs)

	status := profileStatus(report, risk, ruleIDs)
	return ProfileSummary{
		Profile:                 profile,
		DisplayName:             profileDisplayName(profile),
		Status:                  status,
		Summary:                 summarizeProfile(profile, status, report, risk, portabilityCount, ctx.TargetingPosture),
		ErrorCount:              report.ErrorCount(),
		WarningCount:            report.WarningCount(),
		RoutingRisk:             risk.OverallRisk,
		TargetingPosture:        ctx.TargetingPosture,
		PortabilityFindingCount: portabilityCount,
		RuleIDs:                 ruleIDs,
	}
}

func BackendSummaryFromEval(report domaineval.RoutingEvalReport) BackendSummary {
	return BackendSummary{
		BackendID:      report.Backend.ID,
		BackendName:    report.Backend.Name,
		Status:         backendStatus(report.Summary.PassRate, report.Summary.Failed),
		Summary:        summarizeBackend(report.Backend.Name, backendStatus(report.Summary.PassRate, report.Summary.Failed), report.Summary.PassRate, report.Summary.FalsePositives, report.Summary.FalseNegatives, 0),
		PassRate:       report.Summary.PassRate,
		FalsePositives: report.Summary.FalsePositives,
		FalseNegatives: report.Summary.FalseNegatives,
		Failed:         report.Summary.Failed,
	}
}

func BackendSummariesFromMulti(report domaineval.MultiBackendEvalReport) []BackendSummary {
	differingCountByBackend := make(map[string]int)
	for _, item := range report.DifferingCases {
		for _, outcome := range item.Outcomes {
			differingCountByBackend[outcome.BackendID]++
		}
	}

	out := make([]BackendSummary, 0, len(report.Backends))
	for _, backend := range report.Backends {
		status := backendStatus(backend.Summary.PassRate, backend.Summary.Failed)
		out = append(out, BackendSummary{
			BackendID:          backend.Backend.ID,
			BackendName:        backend.Backend.Name,
			Status:             status,
			Summary:            summarizeBackend(backend.Backend.Name, status, backend.Summary.PassRate, backend.Summary.FalsePositives, backend.Summary.FalseNegatives, differingCountByBackend[backend.Backend.ID]),
			PassRate:           backend.Summary.PassRate,
			FalsePositives:     backend.Summary.FalsePositives,
			FalseNegatives:     backend.Summary.FalseNegatives,
			Failed:             backend.Summary.Failed,
			DifferingCaseCount: differingCountByBackend[backend.Backend.ID],
		})
	}
	sortBackends(out)
	return out
}

func classifySupportPosture(profiles []ProfileSummary, backends []BackendSummary) SupportPosture {
	if len(profiles) == 0 {
		return SupportPostureWeakEvidence
	}

	generic, hasGeneric := findProfile(profiles, "generic")
	if !hasGeneric && len(profiles) == 1 {
		return SupportPostureWeakEvidence
	}

	if hasGeneric {
		if hasAnyRule(generic.RuleIDs, lint.RuleAccidentalToolLockIn.ID, lint.RuleGenericPortabilityContradiction.ID, lint.RuleGenericProfileToolLocking.ID) {
			return SupportPostureAccidentalToolLocked
		}
		if hasAnyRule(generic.RuleIDs, lint.RuleMixedEcosystemGuidance.ID, lint.RuleUnclearToolTargeting.ID) {
			return SupportPostureMixedAmbiguous
		}
		if generic.Status == StatusStrong && noRiskyBackends(backends) {
			return SupportPostureGenericPortable
		}
	}

	targetedProfiles := strongestNonGenericProfiles(profiles)
	if len(targetedProfiles) == 1 && targetedProfiles[0].Status == StatusStrong {
		if !hasGeneric || generic.Status != StatusStrong {
			return SupportPostureIntentionalToolSpecific
		}
	}

	if hasGeneric && generic.Status == StatusRisky {
		return SupportPostureAccidentalToolLocked
	}

	if len(targetedProfiles) > 1 {
		return SupportPostureMixedAmbiguous
	}

	return SupportPostureMixedAmbiguous
}

func classifyEvidenceLevel(profiles []ProfileSummary, backends []BackendSummary) EvidenceLevel {
	hasGeneric := false
	for _, profile := range profiles {
		if profile.Profile == "generic" {
			hasGeneric = true
			break
		}
	}
	switch {
	case hasGeneric && len(backends) >= 2:
		return EvidenceLevelStrong
	case hasGeneric || len(profiles) >= 3 || len(backends) >= 1:
		return EvidenceLevelPartial
	default:
		return EvidenceLevelLimited
	}
}

func summarizeReport(posture SupportPosture, level EvidenceLevel, profiles []ProfileSummary, backends []BackendSummary) string {
	switch posture {
	case SupportPostureGenericPortable:
		if len(backends) > 0 {
			return fmt.Sprintf("Firety sees this skill as broadly portable with %s evidence and no major backend-specific compatibility gaps.", level)
		}
		return fmt.Sprintf("Firety sees this skill as broadly portable based on %s compatibility evidence.", level)
	case SupportPostureIntentionalToolSpecific:
		target := "one tool ecosystem"
		if profiles := strongestNonGenericProfiles(profiles); len(profiles) == 1 {
			target = profiles[0].DisplayName
		}
		return fmt.Sprintf("Firety sees this skill as intentionally targeted at %s rather than broadly portable.", target)
	case SupportPostureAccidentalToolLocked:
		return "Firety sees this skill as unintentionally locked to one ecosystem despite broader portability expectations."
	case SupportPostureWeakEvidence:
		return "Firety does not have enough compatibility evidence yet to make a strong support-posture claim."
	default:
		return "Firety sees mixed compatibility signals that are not yet strong enough for a clean portability claim."
	}
}

func summarizeBlockers(posture SupportPosture, profiles []ProfileSummary, backends []BackendSummary) []string {
	values := make([]string, 0, 6)
	switch posture {
	case SupportPostureAccidentalToolLocked:
		values = append(values, "Generic portability claims are contradicted by tool-specific instructions or examples.")
	case SupportPostureMixedAmbiguous:
		values = append(values, "The profile evidence points to mixed or ambiguous targeting rather than one clear support posture.")
	case SupportPostureWeakEvidence:
		values = append(values, "There is not enough profile or backend evidence yet to make strong support claims.")
	}

	for _, profile := range profiles {
		if profile.Status == StatusRisky {
			values = append(values, fmt.Sprintf("%s profile remains risky: %s", profile.DisplayName, profile.Summary))
		}
	}
	for _, backend := range backends {
		if backend.Status == StatusRisky {
			values = append(values, fmt.Sprintf("%s backend is risky: %s", backend.BackendName, backend.Summary))
		}
	}
	return uniqueFirst(values, 5)
}

func summarizeStrengths(posture SupportPosture, profiles []ProfileSummary, backends []BackendSummary) []string {
	values := make([]string, 0, 6)
	if posture == SupportPostureGenericPortable {
		values = append(values, "The generic profile stays low-risk without obvious portability contradictions.")
	}
	for _, profile := range profiles {
		if profile.Status == StatusStrong {
			values = append(values, fmt.Sprintf("%s profile looks healthy: %s", profile.DisplayName, profile.Summary))
		}
	}
	for _, backend := range backends {
		if backend.Status == StatusStrong {
			values = append(values, fmt.Sprintf("%s backend routed all measured cases cleanly.", backend.BackendName))
		}
	}
	return uniqueFirst(values, 5)
}

func recommendedPositioning(posture SupportPosture, profiles []ProfileSummary, backends []BackendSummary) string {
	switch posture {
	case SupportPostureGenericPortable:
		return "Position this skill as broadly portable and keep the main guidance, examples, and install notes tool-neutral."
	case SupportPostureIntentionalToolSpecific:
		target := "the strongest target profile"
		if profiles := strongestNonGenericProfiles(profiles); len(profiles) == 1 {
			target = profiles[0].DisplayName
		}
		return fmt.Sprintf("Position this skill as intentionally targeted to %s and document its audience boundaries explicitly.", target)
	case SupportPostureAccidentalToolLocked:
		return "Do not claim generic portability yet; either rewrite the skill toward neutral wording or explicitly declare the tool ecosystem it really targets."
	case SupportPostureWeakEvidence:
		if len(backends) == 0 {
			return "Collect broader profile or backend evidence before making strong compatibility claims."
		}
		return "Collect broader profile evidence before making strong portability claims."
	default:
		return "Avoid broad support claims until the mixed ecosystem signals are resolved or one intended target is declared clearly."
	}
}

func portabilityRiskSummary(profiles []ProfileSummary) string {
	generic, ok := findProfile(profiles, "generic")
	if !ok {
		return "Generic portability evidence is not available."
	}
	switch generic.Status {
	case StatusStrong:
		return "The generic profile does not show strong portability blockers."
	case StatusRisky:
		return fmt.Sprintf("The generic profile is risky with %d portability finding(s) and %s routing risk.", generic.PortabilityFindingCount, generic.RoutingRisk)
	default:
		return fmt.Sprintf("The generic profile has mixed portability evidence with %d portability finding(s).", generic.PortabilityFindingCount)
	}
}

func routingRiskContext(profiles []ProfileSummary) string {
	if len(profiles) == 0 {
		return ""
	}
	worst := profiles[0]
	for _, profile := range profiles[1:] {
		if routingRiskRank(profile.RoutingRisk) > routingRiskRank(worst.RoutingRisk) {
			worst = profile
		}
	}
	return fmt.Sprintf("The highest routing risk appears under the %s profile at %s.", worst.DisplayName, strings.ToUpper(string(worst.RoutingRisk)))
}

func profileStatus(report lint.Report, risk lint.RoutingRiskSummary, ruleIDs []string) Status {
	switch {
	case report.HasErrors():
		return StatusRisky
	case hasAnyRule(ruleIDs, lint.RuleAccidentalToolLockIn.ID, lint.RuleGenericPortabilityContradiction.ID, lint.RuleProfileTargetMismatch.ID, lint.RuleMixedEcosystemGuidance.ID):
		return StatusRisky
	case risk.OverallRisk == lint.RoutingRiskHigh:
		return StatusRisky
	case risk.OverallRisk == lint.RoutingRiskLow && report.WarningCount() == 0:
		return StatusStrong
	default:
		return StatusMixed
	}
}

func backendStatus(passRate float64, failed int) Status {
	switch {
	case failed == 0:
		return StatusStrong
	case passRate >= 0.75:
		return StatusMixed
	default:
		return StatusRisky
	}
}

func summarizeProfile(profile string, status Status, report lint.Report, risk lint.RoutingRiskSummary, portabilityCount int, posture lint.TargetingPosture) string {
	switch status {
	case StatusStrong:
		return fmt.Sprintf("%s profile has %d warning(s) and %s routing risk without strong portability blockers.", profileDisplayName(profile), report.WarningCount(), risk.OverallRisk)
	case StatusRisky:
		return fmt.Sprintf("%s profile has %d error(s), %d warning(s), %d portability finding(s), and %s routing risk.", profileDisplayName(profile), report.ErrorCount(), report.WarningCount(), portabilityCount, risk.OverallRisk)
	default:
		return fmt.Sprintf("%s profile is mixed: %d warning(s), %d portability finding(s), %s routing risk, posture %s.", profileDisplayName(profile), report.WarningCount(), portabilityCount, risk.OverallRisk, posture)
	}
}

func summarizeBackend(name string, status Status, passRate float64, falsePositives, falseNegatives, differing int) string {
	switch status {
	case StatusStrong:
		return fmt.Sprintf("%.0f%% pass rate with no measured misses.", passRate*100)
	case StatusRisky:
		return fmt.Sprintf("%.0f%% pass rate, %d false positive(s), %d false negative(s), %d differing case(s).", passRate*100, falsePositives, falseNegatives, differing)
	default:
		return fmt.Sprintf("%.0f%% pass rate with some measured misses and %d differing case(s).", passRate*100, differing)
	}
}

func findProfile(profiles []ProfileSummary, profile string) (ProfileSummary, bool) {
	for _, item := range profiles {
		if item.Profile == profile {
			return item, true
		}
	}
	return ProfileSummary{}, false
}

func strongestNonGenericProfiles(profiles []ProfileSummary) []ProfileSummary {
	candidates := make([]ProfileSummary, 0, len(profiles))
	bestScore := -1
	for _, profile := range profiles {
		if profile.Profile == "generic" {
			continue
		}
		score := profileStrengthScore(profile)
		if score > bestScore {
			bestScore = score
			candidates = []ProfileSummary{profile}
			continue
		}
		if score == bestScore {
			candidates = append(candidates, profile)
		}
	}
	sortProfiles(candidates)
	return candidates
}

func profileStrengthScore(profile ProfileSummary) int {
	score := 0
	switch profile.Status {
	case StatusStrong:
		score += 10
	case StatusMixed:
		score += 5
	}
	score -= profile.ErrorCount * 10
	score -= profile.WarningCount
	score -= profile.PortabilityFindingCount * 2
	score -= routingRiskRank(profile.RoutingRisk) * 3
	return score
}

func noRiskyBackends(backends []BackendSummary) bool {
	for _, backend := range backends {
		if backend.Status == StatusRisky {
			return false
		}
	}
	return true
}

func hasAnyRule(ruleIDs []string, targets ...string) bool {
	for _, target := range targets {
		if slices.Contains(ruleIDs, target) {
			return true
		}
	}
	return false
}

func sortProfiles(values []ProfileSummary) {
	order := map[string]int{
		"generic":     0,
		"codex":       1,
		"claude-code": 2,
		"copilot":     3,
		"cursor":      4,
	}
	slices.SortStableFunc(values, func(left, right ProfileSummary) int {
		if order[left.Profile] != order[right.Profile] {
			return order[left.Profile] - order[right.Profile]
		}
		return strings.Compare(left.Profile, right.Profile)
	})
}

func sortBackends(values []BackendSummary) {
	slices.SortStableFunc(values, func(left, right BackendSummary) int {
		if left.Status != right.Status {
			return statusRank(left.Status) - statusRank(right.Status)
		}
		if left.PassRate != right.PassRate {
			if left.PassRate > right.PassRate {
				return -1
			}
			return 1
		}
		return strings.Compare(left.BackendID, right.BackendID)
	})
}

func statusRank(status Status) int {
	switch status {
	case StatusRisky:
		return 0
	case StatusMixed:
		return 1
	case StatusStrong:
		return 2
	default:
		return 3
	}
}

func routingRiskRank(level lint.RoutingRiskLevel) int {
	switch level {
	case lint.RoutingRiskHigh:
		return 2
	case lint.RoutingRiskMedium:
		return 1
	default:
		return 0
	}
}

func profileDisplayName(profile string) string {
	switch profile {
	case "generic":
		return "Generic"
	case "codex":
		return "Codex"
	case "claude-code":
		return "Claude Code"
	case "copilot":
		return "Copilot"
	case "cursor":
		return "Cursor"
	default:
		return profile
	}
}

func uniqueFirst(values []string, limit int) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) == limit {
			break
		}
	}
	return out
}
