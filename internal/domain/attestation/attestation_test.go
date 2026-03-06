package attestation

import (
	"testing"

	"github.com/firety/firety/internal/domain/compatibility"
	domaingate "github.com/firety/firety/internal/domain/gate"
)

func TestBuildReportStrongPortableEvidence(t *testing.T) {
	t.Parallel()

	report := BuildReport(Evidence{
		Target: "skill",
		Compatibility: compatibility.Report{
			Target:         "skill",
			SupportPosture: compatibility.SupportPostureGenericPortable,
			EvidenceLevel:  compatibility.EvidenceLevelStrong,
			Summary:        "Portable summary.",
			Strengths:      []string{"Strong generic coverage."},
			Profiles: []compatibility.ProfileSummary{
				{Profile: "generic", Status: compatibility.StatusStrong},
				{Profile: "codex", Status: compatibility.StatusStrong},
			},
			Backends: []compatibility.BackendSummary{
				{BackendID: "codex", BackendName: "Codex", Status: compatibility.StatusStrong},
			},
		},
		Gate: &domaingate.Result{
			Decision: domaingate.DecisionPass,
			Summary:  "All selected thresholds passed.",
		},
		TestedProfiles: []string{"generic", "generic", "codex"},
		TestedBackends: []compatibility.BackendSummary{
			{BackendID: "codex", BackendName: "Codex", Status: compatibility.StatusStrong, Summary: "100% pass rate."},
		},
		EvidenceRefs: []EvidenceRef{
			{ID: "eval", Kind: "routing-eval", Source: "eval.json"},
			{ID: "compat", Kind: "compatibility", Source: "compat.json"},
			{ID: "gate", Kind: "quality-gate", Source: "gate.json"},
		},
	})

	if report.SupportPosture != compatibility.SupportPostureGenericPortable {
		t.Fatalf("expected generic-portable posture, got %#v", report)
	}
	if report.QualityGate == nil || report.QualityGate.Decision != "pass" {
		t.Fatalf("expected gate summary, got %#v", report.QualityGate)
	}
	if len(report.Claims) < 3 {
		t.Fatalf("expected multiple claims, got %#v", report.Claims)
	}
	if report.Claims[0].Key != "support-posture" {
		t.Fatalf("expected stable claim ordering, got %#v", report.Claims)
	}
	if len(report.TestedProfiles) != 2 || report.TestedProfiles[0] != "codex" || report.TestedProfiles[1] != "generic" {
		t.Fatalf("expected deduplicated sorted tested profiles, got %#v", report.TestedProfiles)
	}
	if len(report.EvidenceRefs) != 3 {
		t.Fatalf("expected normalized evidence refs, got %#v", report.EvidenceRefs)
	}
}

func TestBuildReportWeakEvidenceAddsCaution(t *testing.T) {
	t.Parallel()

	report := BuildReport(Evidence{
		Compatibility: compatibility.Report{
			SupportPosture: compatibility.SupportPostureWeakEvidence,
			EvidenceLevel:  compatibility.EvidenceLevelLimited,
			Summary:        "Not enough evidence.",
		},
	})

	if report.SupportPosture != compatibility.SupportPostureWeakEvidence {
		t.Fatalf("expected weak-evidence posture, got %#v", report)
	}
	if len(report.CautionAreas) == 0 {
		t.Fatalf("expected caution areas, got %#v", report)
	}
	found := false
	for _, claim := range report.Claims {
		if claim.Key == "evidence-limits" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected evidence-limits claim, got %#v", report.Claims)
	}
}
