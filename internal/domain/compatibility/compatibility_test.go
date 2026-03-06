package compatibility

import (
	"testing"

	"github.com/firety/firety/internal/domain/lint"
)

func TestBuildReportGenericPortable(t *testing.T) {
	t.Parallel()

	report := BuildReport(Evidence{
		Target: "skill",
		Profiles: []ProfileSummary{
			{Profile: "generic", DisplayName: "Generic", Status: StatusStrong, RoutingRisk: lint.RoutingRiskLow},
			{Profile: "codex", DisplayName: "Codex", Status: StatusStrong, RoutingRisk: lint.RoutingRiskLow},
		},
		Backends: []BackendSummary{
			{BackendID: "codex", BackendName: "Codex", Status: StatusStrong, PassRate: 1},
			{BackendID: "cursor", BackendName: "Cursor", Status: StatusStrong, PassRate: 1},
		},
	})

	if report.SupportPosture != SupportPostureGenericPortable {
		t.Fatalf("expected generic-portable posture, got %#v", report)
	}
	if report.EvidenceLevel != EvidenceLevelStrong {
		t.Fatalf("expected strong evidence, got %#v", report)
	}
}

func TestBuildReportIntentionalToolSpecific(t *testing.T) {
	t.Parallel()

	report := BuildReport(Evidence{
		Target: "skill",
		Profiles: []ProfileSummary{
			{Profile: "generic", DisplayName: "Generic", Status: StatusMixed, RoutingRisk: lint.RoutingRiskMedium, RuleIDs: []string{lint.RuleToolSpecificBranding.ID}},
			{Profile: "codex", DisplayName: "Codex", Status: StatusStrong, RoutingRisk: lint.RoutingRiskLow},
			{Profile: "cursor", DisplayName: "Cursor", Status: StatusMixed, RoutingRisk: lint.RoutingRiskMedium},
		},
	})

	if report.SupportPosture != SupportPostureIntentionalToolSpecific {
		t.Fatalf("expected intentional tool-specific posture, got %#v", report)
	}
}

func TestBuildReportMixedAmbiguous(t *testing.T) {
	t.Parallel()

	report := BuildReport(Evidence{
		Target: "skill",
		Profiles: []ProfileSummary{
			{Profile: "generic", DisplayName: "Generic", Status: StatusMixed, RoutingRisk: lint.RoutingRiskMedium, RuleIDs: []string{lint.RuleMixedEcosystemGuidance.ID}},
			{Profile: "codex", DisplayName: "Codex", Status: StatusStrong, RoutingRisk: lint.RoutingRiskLow},
			{Profile: "claude-code", DisplayName: "Claude Code", Status: StatusStrong, RoutingRisk: lint.RoutingRiskLow},
		},
	})

	if report.SupportPosture != SupportPostureMixedAmbiguous {
		t.Fatalf("expected mixed posture, got %#v", report)
	}
}

func TestBuildReportAccidentallyToolLocked(t *testing.T) {
	t.Parallel()

	report := BuildReport(Evidence{
		Target: "skill",
		Profiles: []ProfileSummary{
			{Profile: "generic", DisplayName: "Generic", Status: StatusRisky, RoutingRisk: lint.RoutingRiskHigh, RuleIDs: []string{lint.RuleAccidentalToolLockIn.ID}},
			{Profile: "codex", DisplayName: "Codex", Status: StatusStrong, RoutingRisk: lint.RoutingRiskLow},
		},
	})

	if report.SupportPosture != SupportPostureAccidentalToolLocked {
		t.Fatalf("expected accidentally-tool-locked posture, got %#v", report)
	}
}

func TestBuildReportWeakEvidence(t *testing.T) {
	t.Parallel()

	report := BuildReport(Evidence{
		Target: "skill",
		Backends: []BackendSummary{
			{BackendID: "generic", BackendName: "Generic", Status: StatusStrong, PassRate: 1},
		},
	})

	if report.SupportPosture != SupportPostureWeakEvidence {
		t.Fatalf("expected weak-evidence posture, got %#v", report)
	}
	if report.EvidenceLevel != EvidenceLevelPartial {
		t.Fatalf("expected partial evidence from backend-only data, got %#v", report)
	}
}
