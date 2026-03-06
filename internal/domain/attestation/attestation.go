package attestation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/firety/firety/internal/domain/compatibility"
	domaingate "github.com/firety/firety/internal/domain/gate"
)

type Report struct {
	Target                          string                         `json:"target,omitempty"`
	SupportPosture                  compatibility.SupportPosture   `json:"support_posture"`
	EvidenceLevel                   compatibility.EvidenceLevel    `json:"evidence_level"`
	Summary                         string                         `json:"summary"`
	SupportedProfiles               []string                       `json:"supported_profiles,omitempty"`
	TestedProfiles                  []string                       `json:"tested_profiles,omitempty"`
	SupportedBackends               []string                       `json:"supported_backends,omitempty"`
	TestedBackends                  []compatibility.BackendSummary `json:"tested_backends,omitempty"`
	QualityGate                     *GateSummary                   `json:"quality_gate,omitempty"`
	Claims                          []Claim                        `json:"claims,omitempty"`
	Strengths                       []string                       `json:"strengths,omitempty"`
	Limitations                     []string                       `json:"limitations,omitempty"`
	CautionAreas                    []string                       `json:"caution_areas,omitempty"`
	EvidenceRefs                    []EvidenceRef                  `json:"evidence_refs,omitempty"`
	RecommendedConsumerReadingOrder []string                       `json:"recommended_consumer_reading_order,omitempty"`
}

type GateSummary struct {
	Decision string `json:"decision"`
	Summary  string `json:"summary"`
}

type Claim struct {
	Key          string   `json:"key"`
	Statement    string   `json:"statement"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type EvidenceRef struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	ArtifactType string `json:"artifact_type,omitempty"`
	Source       string `json:"source"`
	Summary      string `json:"summary,omitempty"`
}

type Evidence struct {
	Target         string
	Compatibility  compatibility.Report
	Gate           *domaingate.Result
	TestedProfiles []string
	TestedBackends []compatibility.BackendSummary
	EvidenceRefs   []EvidenceRef
}

func BuildReport(evidence Evidence) Report {
	testedProfiles := uniqueSorted(evidence.TestedProfiles)
	testedBackends := normalizeBackends(evidence.TestedBackends)
	supportedProfiles := supportedProfiles(evidence.Compatibility)
	supportedBackends := supportedBackends(evidence.Compatibility)
	limitations := normalizeStrings(evidence.Compatibility.Blockers)
	cautionAreas := cautionAreas(evidence.Compatibility, evidence.Gate, testedProfiles, testedBackends)
	claims := buildClaims(evidence.Compatibility, evidence.Gate, testedProfiles, testedBackends, evidence.EvidenceRefs)
	readingOrder := recommendedReadingOrder(evidence.EvidenceRefs)

	return Report{
		Target:                          firstNonEmpty(evidence.Target, evidence.Compatibility.Target),
		SupportPosture:                  evidence.Compatibility.SupportPosture,
		EvidenceLevel:                   evidence.Compatibility.EvidenceLevel,
		Summary:                         summarize(evidence.Compatibility, evidence.Gate, testedProfiles, testedBackends),
		SupportedProfiles:               supportedProfiles,
		TestedProfiles:                  testedProfiles,
		SupportedBackends:               supportedBackends,
		TestedBackends:                  testedBackends,
		QualityGate:                     gateSummary(evidence.Gate),
		Claims:                          claims,
		Strengths:                       normalizeStrings(evidence.Compatibility.Strengths),
		Limitations:                     limitations,
		CautionAreas:                    cautionAreas,
		EvidenceRefs:                    normalizeRefs(evidence.EvidenceRefs),
		RecommendedConsumerReadingOrder: readingOrder,
	}
}

func summarize(report compatibility.Report, gateResult *domaingate.Result, testedProfiles []string, testedBackends []compatibility.BackendSummary) string {
	var parts []string

	switch report.SupportPosture {
	case compatibility.SupportPostureGenericPortable:
		parts = append(parts, fmt.Sprintf("Firety can credibly back a generic-portable support statement with %s evidence.", report.EvidenceLevel))
	case compatibility.SupportPostureIntentionalToolSpecific:
		parts = append(parts, "Firety can credibly back an intentionally tool-specific support statement.")
	case compatibility.SupportPostureAccidentalToolLocked:
		parts = append(parts, "Firety sees accidental ecosystem lock-in, so broad support claims would be overstated.")
	case compatibility.SupportPostureWeakEvidence:
		parts = append(parts, "Firety does not have enough evidence for a strong support statement yet.")
	default:
		parts = append(parts, "Firety sees mixed compatibility evidence that should be communicated with caution.")
	}

	if len(testedProfiles) > 0 || len(testedBackends) > 0 {
		tested := make([]string, 0, 2)
		if len(testedProfiles) > 0 {
			tested = append(tested, fmt.Sprintf("profiles %s", strings.Join(testedProfiles, ", ")))
		}
		if len(testedBackends) > 0 {
			names := make([]string, 0, len(testedBackends))
			for _, backend := range testedBackends {
				names = append(names, backend.BackendName)
			}
			tested = append(tested, fmt.Sprintf("backends %s", strings.Join(names, ", ")))
		}
		parts = append(parts, fmt.Sprintf("Measured routing evidence covers %s.", strings.Join(tested, " and ")))
	}

	if gateResult != nil {
		switch gateResult.Decision {
		case domaingate.DecisionPass:
			parts = append(parts, "The selected quality gate passed.")
		case domaingate.DecisionFail:
			parts = append(parts, "The selected quality gate did not pass.")
		}
	}

	return strings.Join(parts, " ")
}

func gateSummary(result *domaingate.Result) *GateSummary {
	if result == nil {
		return nil
	}
	return &GateSummary{
		Decision: string(result.Decision),
		Summary:  result.Summary,
	}
}

func buildClaims(report compatibility.Report, gateResult *domaingate.Result, testedProfiles []string, testedBackends []compatibility.BackendSummary, refs []EvidenceRef) []Claim {
	claims := make([]Claim, 0, 5)

	compatRefs := refIDsByKind(refs, "compatibility")
	gateRefs := refIDsByKind(refs, "quality-gate")
	evalRefs := refIDsByKind(refs, "routing-eval")

	claims = append(claims, Claim{
		Key:          "support-posture",
		Statement:    supportPostureStatement(report),
		EvidenceRefs: compatRefs,
	})

	if len(testedProfiles) > 0 || len(testedBackends) > 0 {
		statement := "Firety measured routing behavior for "
		segments := make([]string, 0, 2)
		if len(testedProfiles) > 0 {
			segments = append(segments, fmt.Sprintf("profiles %s", strings.Join(testedProfiles, ", ")))
		}
		if len(testedBackends) > 0 {
			names := make([]string, 0, len(testedBackends))
			for _, backend := range testedBackends {
				names = append(names, backend.BackendName)
			}
			segments = append(segments, fmt.Sprintf("backends %s", strings.Join(names, ", ")))
		}
		statement += strings.Join(segments, " and ") + "."
		claims = append(claims, Claim{
			Key:          "tested-routing-scope",
			Statement:    statement,
			EvidenceRefs: evalRefs,
		})
	}

	if gateResult != nil {
		statement := "The selected quality gate passed."
		if gateResult.Decision == domaingate.DecisionFail {
			statement = "The selected quality gate did not pass."
		}
		claims = append(claims, Claim{
			Key:          "quality-gate",
			Statement:    statement,
			EvidenceRefs: gateRefs,
		})
	}

	if report.SupportPosture == compatibility.SupportPostureGenericPortable && len(report.Blockers) == 0 {
		claims = append(claims, Claim{
			Key:          "portability-clean",
			Statement:    "No major portability blockers were detected for the supported profiles Firety evaluated.",
			EvidenceRefs: compatRefs,
		})
	}

	if report.EvidenceLevel != compatibility.EvidenceLevelStrong {
		claims = append(claims, Claim{
			Key:          "evidence-limits",
			Statement:    "Support claims should be read with caution because Firety's current evidence is partial or limited.",
			EvidenceRefs: compatRefs,
		})
	}

	return claims
}

func supportPostureStatement(report compatibility.Report) string {
	switch report.SupportPosture {
	case compatibility.SupportPostureGenericPortable:
		return "Firety sees this skill as generic-portable, with caveats limited to any explicit caution areas below."
	case compatibility.SupportPostureIntentionalToolSpecific:
		return "Firety sees this skill as intentionally tool-specific rather than broadly portable."
	case compatibility.SupportPostureAccidentalToolLocked:
		return "Firety sees this skill as accidentally tool-locked, so broad support claims would be misleading."
	case compatibility.SupportPostureWeakEvidence:
		return "Firety does not have enough evidence yet for a strong support claim."
	default:
		return "Firety sees mixed support signals, so maintainers should avoid over-broad support claims."
	}
}

func supportedProfiles(report compatibility.Report) []string {
	values := make([]string, 0, len(report.Profiles))
	for _, profile := range report.Profiles {
		if profile.Status != compatibility.StatusStrong {
			continue
		}
		values = append(values, profile.Profile)
	}
	return uniqueSorted(values)
}

func supportedBackends(report compatibility.Report) []string {
	values := make([]string, 0, len(report.Backends))
	for _, backend := range report.Backends {
		if backend.Status != compatibility.StatusStrong {
			continue
		}
		values = append(values, backend.BackendID)
	}
	return uniqueSorted(values)
}

func cautionAreas(report compatibility.Report, gateResult *domaingate.Result, testedProfiles []string, testedBackends []compatibility.BackendSummary) []string {
	values := make([]string, 0, 8)
	values = append(values, report.Blockers...)

	if report.EvidenceLevel != compatibility.EvidenceLevelStrong {
		values = append(values, "Evidence coverage is partial or limited, so support claims should stay conservative.")
	}
	if len(testedProfiles) == 0 && len(testedBackends) == 0 {
		values = append(values, "No measured routing eval evidence was provided.")
	}
	for _, backend := range testedBackends {
		if backend.Status == compatibility.StatusRisky || backend.Status == compatibility.StatusMixed {
			values = append(values, backend.Summary)
		}
	}
	if gateResult != nil && gateResult.Decision == domaingate.DecisionFail {
		values = append(values, gateResult.Summary)
	}

	return normalizeStrings(values)
}

func recommendedReadingOrder(refs []EvidenceRef) []string {
	order := make([]string, 0, len(refs))
	for _, kind := range []string{"quality-gate", "compatibility", "routing-eval", "lint"} {
		for _, ref := range refs {
			if ref.Kind != kind {
				continue
			}
			order = append(order, ref.Source)
		}
	}
	return uniqueSorted(order)
}

func refIDsByKind(refs []EvidenceRef, kind string) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Kind != kind {
			continue
		}
		out = append(out, ref.ID)
	}
	return out
}

func normalizeBackends(values []compatibility.BackendSummary) []compatibility.BackendSummary {
	out := append([]compatibility.BackendSummary(nil), values...)
	slices.SortFunc(out, func(a, b compatibility.BackendSummary) int {
		return strings.Compare(a.BackendID, b.BackendID)
	})
	return out
}

func normalizeRefs(values []EvidenceRef) []EvidenceRef {
	out := append([]EvidenceRef(nil), values...)
	slices.SortFunc(out, func(a, b EvidenceRef) int {
		if compare := strings.Compare(a.Kind, b.Kind); compare != 0 {
			return compare
		}
		return strings.Compare(a.Source, b.Source)
	})
	return out
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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

func uniqueSorted(values []string) []string {
	out := normalizeStrings(values)
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
