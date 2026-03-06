package readiness_test

import (
	"testing"

	"github.com/firety/firety/internal/domain/attestation"
	"github.com/firety/firety/internal/domain/compatibility"
	"github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/readiness"
)

func TestBuildReadyForMergeWithFreshEvidence(t *testing.T) {
	t.Parallel()

	result := readiness.Build(readiness.Evidence{
		Target:  "skill",
		Context: readiness.ContextMerge,
		Gate: &gate.Result{
			Decision: gate.DecisionPass,
			Summary:  "Gate passed.",
		},
		Compatibility: &compatibility.Report{
			Target:         "skill",
			SupportPosture: compatibility.SupportPostureGenericPortable,
			EvidenceLevel:  compatibility.EvidenceLevelStrong,
			Summary:        "Portable and healthy.",
		},
		Attestation: &attestation.Report{
			Target:         "skill",
			SupportPosture: compatibility.SupportPostureGenericPortable,
			EvidenceLevel:  compatibility.EvidenceLevelStrong,
			Summary:        "Strong support statement.",
			TestedProfiles: []string{"generic"},
		},
		Freshness: &readiness.FreshnessSummary{
			Status:     readiness.FreshnessFresh,
			AgeSummary: "Fresh.",
		},
	})

	if result.Decision != readiness.DecisionReady {
		t.Fatalf("expected ready, got %#v", result)
	}
	if len(result.Blockers) != 0 || len(result.Caveats) != 0 {
		t.Fatalf("expected no blockers or caveats, got %#v", result)
	}
}

func TestBuildReadyWithCaveatsForInternalWithoutGate(t *testing.T) {
	t.Parallel()

	result := readiness.Build(readiness.Evidence{
		Target:  "skill",
		Context: readiness.ContextInternal,
		Compatibility: &compatibility.Report{
			Target:         "skill",
			SupportPosture: compatibility.SupportPostureMixedAmbiguous,
			EvidenceLevel:  compatibility.EvidenceLevelPartial,
			Summary:        "Mixed evidence.",
			Blockers:       []string{"Mixed ecosystem wording remains."},
		},
	})

	if result.Decision != readiness.DecisionReadyWithCaveats {
		t.Fatalf("expected ready-with-caveats, got %#v", result)
	}
	if len(result.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %#v", result.Blockers)
	}
	if len(result.Caveats) == 0 {
		t.Fatalf("expected caveats, got %#v", result)
	}
}

func TestBuildNotReadyWhenGateFails(t *testing.T) {
	t.Parallel()

	result := readiness.Build(readiness.Evidence{
		Target:  "skill",
		Context: readiness.ContextReleaseCandidate,
		Gate: &gate.Result{
			Decision: gate.DecisionFail,
			Summary:  "Gate failed on eval regressions.",
		},
		Compatibility: &compatibility.Report{
			Target:         "skill",
			SupportPosture: compatibility.SupportPostureGenericPortable,
			EvidenceLevel:  compatibility.EvidenceLevelStrong,
			Summary:        "Portable and healthy.",
		},
		Freshness: &readiness.FreshnessSummary{
			Status:     readiness.FreshnessFresh,
			AgeSummary: "Fresh.",
		},
	})

	if result.Decision != readiness.DecisionNotReady {
		t.Fatalf("expected not-ready, got %#v", result)
	}
	if len(result.Blockers) == 0 {
		t.Fatalf("expected blockers, got %#v", result)
	}
}

func TestBuildInsufficientEvidenceForPublicAttestation(t *testing.T) {
	t.Parallel()

	result := readiness.Build(readiness.Evidence{
		Target:  "skill",
		Context: readiness.ContextPublicAttestation,
	})

	if result.Decision != readiness.DecisionInsufficient {
		t.Fatalf("expected insufficient-evidence, got %#v", result)
	}
	if len(result.Blockers) == 0 {
		t.Fatalf("expected blockers, got %#v", result)
	}
}
