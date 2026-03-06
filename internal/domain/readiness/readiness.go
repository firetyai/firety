package readiness

import (
	"fmt"
	"slices"
	"strings"

	"github.com/firety/firety/internal/domain/attestation"
	"github.com/firety/firety/internal/domain/compatibility"
	"github.com/firety/firety/internal/domain/gate"
)

type PublishContext string
type Decision string
type Suitability string
type FreshnessStatus string

const (
	ContextInternal          PublishContext = "internal"
	ContextMerge             PublishContext = "merge"
	ContextReleaseCandidate  PublishContext = "release-candidate"
	ContextPublicRelease     PublishContext = "public-release"
	ContextPublicAttestation PublishContext = "public-attestation"
	ContextPublicTrustReport PublishContext = "public-trust-report"

	DecisionReady            Decision = "ready"
	DecisionReadyWithCaveats Decision = "ready-with-caveats"
	DecisionNotReady         Decision = "not-ready"
	DecisionInsufficient     Decision = "insufficient-evidence"

	SuitabilitySuitable             Suitability = "suitable"
	SuitabilityUsableWithCaveats    Suitability = "usable-with-caveats"
	SuitabilityNotSuitable          Suitability = "not-suitable"
	SuitabilityInsufficientEvidence Suitability = "insufficient-evidence"

	FreshnessFresh                FreshnessStatus = "fresh"
	FreshnessUsableWithCaveats    FreshnessStatus = "usable-with-caveats"
	FreshnessStale                FreshnessStatus = "stale"
	FreshnessInsufficientEvidence FreshnessStatus = "insufficient-evidence"
)

type Reason struct {
	Code                   string   `json:"code"`
	Title                  string   `json:"title"`
	Summary                string   `json:"summary"`
	SupportingArtifactRefs []string `json:"supporting_artifact_refs,omitempty"`
	RelatedRuleIDs         []string `json:"related_rule_ids,omitempty"`
	RelatedEvalCaseIDs     []string `json:"related_eval_case_ids,omitempty"`
	RelatedBackendIDs      []string `json:"related_backend_ids,omitempty"`
}

type EvidenceSummary struct {
	GateDecision       string                       `json:"gate_decision,omitempty"`
	FreshnessStatus    FreshnessStatus              `json:"freshness_status,omitempty"`
	SupportPosture     compatibility.SupportPosture `json:"support_posture,omitempty"`
	EvidenceLevel      compatibility.EvidenceLevel  `json:"evidence_level,omitempty"`
	AttestationSummary string                       `json:"attestation_summary,omitempty"`
	TrustReportSummary string                       `json:"trust_report_summary,omitempty"`
}

type FreshnessSummary struct {
	Status                 FreshnessStatus `json:"status"`
	AgeSummary             string          `json:"age_summary"`
	Caveats                []string        `json:"caveats,omitempty"`
	RecertificationActions []string        `json:"recertification_actions,omitempty"`
	SupportingPaths        []string        `json:"supporting_paths,omitempty"`
}

type SurfaceSummary struct {
	Suitability Suitability `json:"suitability"`
	Summary     string      `json:"summary"`
}

type Result struct {
	SchemaVersion            string                `json:"schema_version"`
	Target                   string                `json:"target,omitempty"`
	PublishContext           PublishContext        `json:"publish_context"`
	Decision                 Decision              `json:"decision"`
	Summary                  string                `json:"summary"`
	Blockers                 []Reason              `json:"blockers,omitempty"`
	Caveats                  []Reason              `json:"caveats,omitempty"`
	ImprovementOpportunities []string              `json:"improvement_opportunities,omitempty"`
	EvidenceSummary          EvidenceSummary       `json:"evidence_summary"`
	FreshnessSummary         *FreshnessSummary     `json:"freshness_summary,omitempty"`
	GateSummary              *gate.Result          `json:"gate_summary,omitempty"`
	SupportPostureSummary    *compatibility.Report `json:"support_posture_summary,omitempty"`
	AttestationSuitability   SurfaceSummary        `json:"attestation_publishability"`
	TrustReportSuitability   SurfaceSummary        `json:"trust_report_publishability"`
	RecommendedActions       []string              `json:"recommended_actions,omitempty"`
	SupportingArtifactRefs   []string              `json:"supporting_artifact_refs,omitempty"`
}

type Evidence struct {
	Target        string
	Context       PublishContext
	Gate          *gate.Result
	Compatibility *compatibility.Report
	Attestation   *attestation.Report
	Freshness     *FreshnessSummary
	ArtifactRefs  []string
}

func ParsePublishContext(raw string) (PublishContext, error) {
	switch PublishContext(strings.TrimSpace(raw)) {
	case ContextInternal, ContextMerge, ContextReleaseCandidate, ContextPublicRelease, ContextPublicAttestation, ContextPublicTrustReport:
		return PublishContext(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("invalid publish context %q: must be one of %s, %s, %s, %s, %s, %s", raw, ContextInternal, ContextMerge, ContextReleaseCandidate, ContextPublicRelease, ContextPublicAttestation, ContextPublicTrustReport)
	}
}

func Build(evidence Evidence) Result {
	result := Result{
		SchemaVersion:  "1",
		Target:         firstNonEmpty(evidence.Target, compatibilityTarget(evidence.Compatibility), attestationTarget(evidence.Attestation)),
		PublishContext: evidence.Context,
		EvidenceSummary: EvidenceSummary{
			GateDecision:    gateDecision(evidence.Gate),
			FreshnessStatus: freshnessStatus(evidence.Freshness),
			SupportPosture:  compatibilityPosture(evidence.Compatibility),
			EvidenceLevel:   compatibilityLevel(evidence.Compatibility),
		},
		FreshnessSummary:       evidence.Freshness,
		GateSummary:            evidence.Gate,
		SupportPostureSummary:  evidence.Compatibility,
		AttestationSuitability: attestationSuitability(evidence),
		TrustReportSuitability: trustReportSuitability(evidence),
		SupportingArtifactRefs: uniqueSortedStrings(evidence.ArtifactRefs),
	}
	if evidence.Attestation != nil {
		result.EvidenceSummary.AttestationSummary = evidence.Attestation.Summary
	}
	result.EvidenceSummary.TrustReportSummary = result.TrustReportSuitability.Summary

	blockers := make([]Reason, 0, 8)
	caveats := make([]Reason, 0, 8)
	opportunities := make([]string, 0, 8)
	actions := make([]string, 0, 8)

	if evidence.Gate == nil {
		maybeAdd(&blockers, &caveats, reasonForGateMissing(evidence.Context))
		actions = append(actions, actionForGateMissing(evidence.Context))
	} else if evidence.Gate.Decision == gate.DecisionFail {
		blockers = append(blockers, Reason{
			Code:  "gate-failed",
			Title: "Quality gate failed",
			Summary: firstNonEmpty(
				evidence.Gate.Summary,
				"The selected Firety quality gate did not pass.",
			),
			SupportingArtifactRefs: uniqueSortedStrings(evidence.ArtifactRefs),
			RelatedRuleIDs:         relatedRuleIDsFromGate(evidence.Gate),
			RelatedEvalCaseIDs:     relatedEvalCasesFromGate(evidence.Gate),
		})
		actions = append(actions, "Resolve the blocking quality-gate reasons before publishing or releasing.")
	}

	if evidence.Freshness == nil {
		maybeAdd(&blockers, &caveats, reasonForFreshnessMissing(evidence.Context))
		actions = append(actions, "Capture fresh Firety evidence or inspect an existing artifact with provenance before reusing it for publishing.")
	} else {
		applyFreshnessReasoning(evidence.Context, *evidence.Freshness, &blockers, &caveats, &actions)
	}

	if evidence.Compatibility == nil {
		maybeAdd(&blockers, &caveats, reasonForCompatibilityMissing(evidence.Context))
		actions = append(actions, "Generate compatibility evidence before making support or publishing decisions.")
	} else {
		applyCompatibilityReasoning(evidence.Context, *evidence.Compatibility, &blockers, &caveats, &opportunities, &actions)
	}

	applySurfaceReasoning(
		evidence.Context,
		result.AttestationSuitability,
		"attestation",
		&blockers,
		&caveats,
		&actions,
	)
	applySurfaceReasoning(
		evidence.Context,
		result.TrustReportSuitability,
		"trust-report",
		&blockers,
		&caveats,
		&actions,
	)

	blockers = normalizeReasons(blockers)
	caveats = normalizeReasons(caveats)
	opportunities = uniqueSortedStrings(opportunities)
	actions = uniqueSortedStrings(actions)

	result.Blockers = blockers
	result.Caveats = caveats
	result.ImprovementOpportunities = firstNStrings(opportunities, 5)
	result.RecommendedActions = firstNStrings(actions, 5)
	result.Decision = classifyDecision(evidence.Context, blockers, caveats)
	result.Summary = summarize(result)
	return result
}

func reasonForGateMissing(context PublishContext) reasonClassification {
	reason := Reason{
		Code:    "missing-gate-evidence",
		Title:   "Missing quality-gate evidence",
		Summary: "There is no Firety quality-gate result to support this readiness decision.",
	}
	switch context {
	case ContextInternal:
		return reasonClassification{kind: "caveat", reason: reason}
	default:
		return reasonClassification{kind: "insufficient", reason: reason}
	}
}

func reasonForFreshnessMissing(context PublishContext) reasonClassification {
	reason := Reason{
		Code:    "missing-freshness-evidence",
		Title:   "Missing freshness evidence",
		Summary: "Firety cannot tell whether the supporting evidence is still current enough to reuse.",
	}
	switch context {
	case ContextInternal:
		return reasonClassification{kind: "caveat", reason: reason}
	default:
		return reasonClassification{kind: "insufficient", reason: reason}
	}
}

func reasonForCompatibilityMissing(context PublishContext) reasonClassification {
	reason := Reason{
		Code:    "missing-compatibility-evidence",
		Title:   "Missing compatibility evidence",
		Summary: "Firety does not have a compatibility or support-posture summary for this readiness decision.",
	}
	switch context {
	case ContextInternal:
		return reasonClassification{kind: "caveat", reason: reason}
	default:
		return reasonClassification{kind: "insufficient", reason: reason}
	}
}

func applyFreshnessReasoning(context PublishContext, report FreshnessSummary, blockers, caveats *[]Reason, actions *[]string) {
	switch report.Status {
	case FreshnessFresh:
		return
	case FreshnessUsableWithCaveats:
		*caveats = append(*caveats, Reason{
			Code:                   "freshness-caveat",
			Title:                  "Evidence freshness has caveats",
			Summary:                summarizeFreshness(report),
			SupportingArtifactRefs: uniqueSortedStrings(report.SupportingPaths),
		})
		*actions = append(*actions, report.RecertificationActions...)
	case FreshnessStale:
		target := blockers
		if context == ContextInternal {
			target = caveats
		}
		*target = append(*target, Reason{
			Code:                   "stale-evidence",
			Title:                  "Saved evidence is stale",
			Summary:                summarizeFreshness(report),
			SupportingArtifactRefs: uniqueSortedStrings(report.SupportingPaths),
		})
		*actions = append(*actions, report.RecertificationActions...)
	case FreshnessInsufficientEvidence:
		*blockers = append(*blockers, Reason{
			Code:                   "incomplete-freshness-evidence",
			Title:                  "Freshness evidence is incomplete",
			Summary:                summarizeFreshness(report),
			SupportingArtifactRefs: uniqueSortedStrings(report.SupportingPaths),
		})
		*actions = append(*actions, report.RecertificationActions...)
	}
}

func applyCompatibilityReasoning(context PublishContext, report compatibility.Report, blockers, caveats *[]Reason, opportunities, actions *[]string) {
	switch report.SupportPosture {
	case compatibility.SupportPostureWeakEvidence:
		reason := Reason{
			Code:    "weak-support-evidence",
			Title:   "Support posture evidence is weak",
			Summary: firstNonEmpty(report.Summary, "Firety does not have enough compatibility evidence for a strong support posture."),
		}
		if context == ContextPublicAttestation {
			*blockers = append(*blockers, reason)
			*actions = append(*actions, "Gather stronger profile and backend evidence before publishing support claims.")
		} else {
			*caveats = append(*caveats, reason)
			*actions = append(*actions, "Gather stronger profile or backend compatibility evidence before broadening support claims.")
		}
	case compatibility.SupportPostureAccidentalToolLocked:
		*caveats = append(*caveats, Reason{
			Code:    "accidental-tool-lock-in",
			Title:   "Support posture is accidentally tool-locked",
			Summary: firstNonEmpty(report.Summary, "Firety sees accidental ecosystem lock-in, so support claims should stay narrow."),
		})
		*actions = append(*actions, "Narrow the published support claim or reduce accidental ecosystem lock-in.")
	case compatibility.SupportPostureMixedAmbiguous:
		*caveats = append(*caveats, Reason{
			Code:    "mixed-support-posture",
			Title:   "Support posture is mixed",
			Summary: firstNonEmpty(report.Summary, "Firety sees mixed compatibility evidence that should be communicated with caution."),
		})
		*actions = append(*actions, "Clarify the target tool posture or publish caveated support language.")
	}

	for _, blocker := range firstNStrings(report.Blockers, 3) {
		*caveats = append(*caveats, Reason{
			Code:    "support-blocker",
			Title:   "Known support limitation",
			Summary: blocker,
		})
	}
	*opportunities = append(*opportunities, report.Strengths...)
	if report.RecommendedPositioning != "" {
		*actions = append(*actions, report.RecommendedPositioning)
	}
}

func attestationSuitability(evidence Evidence) SurfaceSummary {
	if evidence.Compatibility == nil {
		return SurfaceSummary{
			Suitability: SuitabilityInsufficientEvidence,
			Summary:     "Attestation suitability cannot be established without compatibility evidence.",
		}
	}
	if evidence.Freshness != nil && (evidence.Freshness.Status == FreshnessStale || evidence.Freshness.Status == FreshnessInsufficientEvidence) {
		return SurfaceSummary{
			Suitability: SuitabilityNotSuitable,
			Summary:     "Attestation evidence should be recertified before making publishable support claims.",
		}
	}
	if evidence.Gate != nil && evidence.Gate.Decision == gate.DecisionFail {
		return SurfaceSummary{
			Suitability: SuitabilityNotSuitable,
			Summary:     "Attestation should not be published while the quality gate is failing.",
		}
	}
	report := evidence.Attestation
	if report == nil {
		switch evidence.Compatibility.EvidenceLevel {
		case compatibility.EvidenceLevelStrong:
			return SurfaceSummary{
				Suitability: SuitabilitySuitable,
				Summary:     "Current Firety evidence is strong enough to support a conservative attestation.",
			}
		case compatibility.EvidenceLevelPartial:
			return SurfaceSummary{
				Suitability: SuitabilityUsableWithCaveats,
				Summary:     "An attestation is possible, but it should clearly state scope, tested areas, and caution areas.",
			}
		default:
			return SurfaceSummary{
				Suitability: SuitabilityInsufficientEvidence,
				Summary:     "Firety does not have enough evidence for a credible support attestation yet.",
			}
		}
	}
	if report.EvidenceLevel == compatibility.EvidenceLevelLimited {
		return SurfaceSummary{
			Suitability: SuitabilityUsableWithCaveats,
			Summary:     "Attestation is possible only with conservative claims because the evidence remains limited.",
		}
	}
	if len(report.TestedProfiles) == 0 && len(report.TestedBackends) == 0 {
		return SurfaceSummary{
			Suitability: SuitabilityUsableWithCaveats,
			Summary:     "Attestation can be published only with clear tested-vs-supported caveats because measured routing evidence is missing.",
		}
	}
	return SurfaceSummary{
		Suitability: SuitabilitySuitable,
		Summary:     "Attestation evidence is current enough for a conservative publishable support statement.",
	}
}

func trustReportSuitability(evidence Evidence) SurfaceSummary {
	if evidence.Compatibility == nil {
		return SurfaceSummary{
			Suitability: SuitabilityInsufficientEvidence,
			Summary:     "Trust-report suitability cannot be established without compatibility evidence.",
		}
	}
	if evidence.Freshness != nil && evidence.Freshness.Status == FreshnessInsufficientEvidence {
		return SurfaceSummary{
			Suitability: SuitabilityInsufficientEvidence,
			Summary:     "Trust-report evidence is incomplete, so a publishable report would be hard to trust.",
		}
	}
	if evidence.Freshness != nil && evidence.Freshness.Status == FreshnessStale {
		return SurfaceSummary{
			Suitability: SuitabilityNotSuitable,
			Summary:     "Trust-report evidence is stale and should be rebuilt before publication.",
		}
	}
	if evidence.Gate != nil && evidence.Gate.Decision == gate.DecisionFail {
		return SurfaceSummary{
			Suitability: SuitabilityUsableWithCaveats,
			Summary:     "A trust report can still be published, but it should explicitly call out the failing quality gate.",
		}
	}
	if evidence.Compatibility.EvidenceLevel == compatibility.EvidenceLevelLimited {
		return SurfaceSummary{
			Suitability: SuitabilityUsableWithCaveats,
			Summary:     "A trust report is possible, but it should foreground weak evidence and known limitations.",
		}
	}
	return SurfaceSummary{
		Suitability: SuitabilitySuitable,
		Summary:     "Current Firety evidence is sufficient for a publishable static trust report.",
	}
}

func applySurfaceReasoning(context PublishContext, summary SurfaceSummary, subject string, blockers, caveats *[]Reason, actions *[]string) {
	title := "Readiness surface is limited"
	switch subject {
	case "attestation":
		title = "Attestation is not publishable yet"
	case "trust-report":
		title = "Trust report is not publishable yet"
	}

	switch summary.Suitability {
	case SuitabilityNotSuitable:
		if context == ContextPublicAttestation && subject == "attestation" ||
			context == ContextPublicTrustReport && subject == "trust-report" ||
			context == ContextPublicRelease && subject == "attestation" {
			*blockers = append(*blockers, Reason{
				Code:    subject + "-not-suitable",
				Title:   title,
				Summary: summary.Summary,
			})
		} else {
			*caveats = append(*caveats, Reason{
				Code:    subject + "-not-suitable",
				Title:   title,
				Summary: summary.Summary,
			})
		}
		*actions = append(*actions, actionForSurface(subject))
	case SuitabilityInsufficientEvidence:
		if context == ContextPublicAttestation && subject == "attestation" ||
			context == ContextPublicTrustReport && subject == "trust-report" {
			*blockers = append(*blockers, Reason{
				Code:    subject + "-insufficient-evidence",
				Title:   title,
				Summary: summary.Summary,
			})
		} else {
			*caveats = append(*caveats, Reason{
				Code:    subject + "-insufficient-evidence",
				Title:   title,
				Summary: summary.Summary,
			})
		}
		*actions = append(*actions, actionForSurface(subject))
	case SuitabilityUsableWithCaveats:
		if context == ContextPublicAttestation && subject == "attestation" {
			*caveats = append(*caveats, Reason{
				Code:    subject + "-caveat",
				Title:   "Attestation needs caveated claims",
				Summary: summary.Summary,
			})
		} else if context == ContextPublicTrustReport && subject == "trust-report" {
			*caveats = append(*caveats, Reason{
				Code:    subject + "-caveat",
				Title:   "Trust report needs caveated wording",
				Summary: summary.Summary,
			})
		}
	}
}

func classifyDecision(context PublishContext, blockers, caveats []Reason) Decision {
	if len(blockers) > 0 {
		if allBlockersAreInsufficient(blockers) {
			return DecisionInsufficient
		}
		return DecisionNotReady
	}
	if len(caveats) > 0 {
		return DecisionReadyWithCaveats
	}
	return DecisionReady
}

func summarize(result Result) string {
	switch result.Decision {
	case DecisionReady:
		return fmt.Sprintf("Ready for %s based on the current Firety evidence.", result.PublishContext)
	case DecisionReadyWithCaveats:
		return fmt.Sprintf("Ready for %s with caveats that should be communicated or reviewed first.", result.PublishContext)
	case DecisionInsufficient:
		return fmt.Sprintf("Not enough current Firety evidence is available to make a reliable %s decision yet.", result.PublishContext)
	default:
		return fmt.Sprintf("Not ready for %s because Firety found blocking quality or evidence issues.", result.PublishContext)
	}
}

type reasonClassification struct {
	kind   string
	reason Reason
}

func maybeAdd(blockers, caveats *[]Reason, classification reasonClassification) {
	switch classification.kind {
	case "caveat":
		*caveats = append(*caveats, classification.reason)
	default:
		*blockers = append(*blockers, classification.reason)
	}
}

func actionForGateMissing(context PublishContext) string {
	if context == ContextInternal {
		return "Run a Firety quality gate before using this result for merge or release decisions."
	}
	return "Run a Firety quality gate before using this result for publishing or release decisions."
}

func actionForSurface(subject string) string {
	switch subject {
	case "attestation":
		return "Regenerate the attestation from fresh evidence before publishing support claims."
	case "trust-report":
		return "Rebuild the trust report from fresh evidence before publishing it."
	default:
		return "Refresh the supporting Firety evidence before publishing."
	}
}

func summarizeFreshness(report FreshnessSummary) string {
	if report.AgeSummary == "" {
		return firstNonEmpty(strings.Join(report.Caveats, "; "), "Freshness evidence has unresolved caveats.")
	}
	if len(report.Caveats) > 0 {
		return fmt.Sprintf("%s Caveats: %s.", report.AgeSummary, strings.Join(report.Caveats, "; "))
	}
	return report.AgeSummary
}

func gateDecision(result *gate.Result) string {
	if result == nil {
		return ""
	}
	return string(result.Decision)
}

func freshnessStatus(report *FreshnessSummary) FreshnessStatus {
	if report == nil {
		return ""
	}
	return report.Status
}

func compatibilityPosture(report *compatibility.Report) compatibility.SupportPosture {
	if report == nil {
		return ""
	}
	return report.SupportPosture
}

func compatibilityLevel(report *compatibility.Report) compatibility.EvidenceLevel {
	if report == nil {
		return ""
	}
	return report.EvidenceLevel
}

func compatibilityTarget(report *compatibility.Report) string {
	if report == nil {
		return ""
	}
	return report.Target
}

func attestationTarget(report *attestation.Report) string {
	if report == nil {
		return ""
	}
	return report.Target
}

func relatedRuleIDsFromGate(result *gate.Result) []string {
	values := make([]string, 0, 8)
	for _, reason := range result.BlockingReasons {
		values = append(values, reason.RelatedRuleIDs...)
	}
	return uniqueSortedStrings(values)
}

func relatedEvalCasesFromGate(result *gate.Result) []string {
	values := make([]string, 0, 8)
	for _, reason := range result.BlockingReasons {
		values = append(values, reason.RelatedEvalCaseIDs...)
	}
	return uniqueSortedStrings(values)
}

func normalizeReasons(values []Reason) []Reason {
	seen := make(map[string]Reason, len(values))
	order := make([]string, 0, len(values))
	for _, value := range values {
		key := value.Code + "|" + value.Title + "|" + value.Summary
		if _, ok := seen[key]; ok {
			continue
		}
		value.SupportingArtifactRefs = uniqueSortedStrings(value.SupportingArtifactRefs)
		value.RelatedRuleIDs = uniqueSortedStrings(value.RelatedRuleIDs)
		value.RelatedEvalCaseIDs = uniqueSortedStrings(value.RelatedEvalCaseIDs)
		value.RelatedBackendIDs = uniqueSortedStrings(value.RelatedBackendIDs)
		seen[key] = value
		order = append(order, key)
	}
	slices.Sort(order)
	out := make([]Reason, 0, len(order))
	for _, key := range order {
		out = append(out, seen[key])
	}
	return out
}

func allBlockersAreInsufficient(values []Reason) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !strings.Contains(value.Code, "insufficient") && !strings.Contains(value.Code, "missing") {
			return false
		}
	}
	return true
}

func firstNStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
