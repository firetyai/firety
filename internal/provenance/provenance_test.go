package provenance

import "testing"

func TestCompareComparable(t *testing.T) {
	t.Parallel()

	base := Inspection{
		Path:          "/tmp/base.json",
		Kind:          ObjectKindArtifact,
		Type:          "firety.skill-lint",
		SchemaVersion: "1",
		Provenance: NormalizeRecord(Record{
			CommandOrigin: "firety skill lint",
			Profile:       "generic",
			Strictness:    "default",
			FailOn:        "errors",
		}),
	}
	candidate := Inspection{
		Path:          "/tmp/candidate.json",
		Kind:          ObjectKindArtifact,
		Type:          "firety.skill-lint",
		SchemaVersion: "1",
		Provenance: NormalizeRecord(Record{
			CommandOrigin: "firety skill lint",
			Profile:       "generic",
			Strictness:    "default",
			FailOn:        "errors",
		}),
	}

	result := Compare(base, candidate)
	if result.Status != CompareComparable {
		t.Fatalf("expected comparable, got %#v", result)
	}
	if result.RerunRecommendation == "" {
		t.Fatalf("expected rerun guidance, got %#v", result)
	}
}

func TestComparePartiallyComparableWhenSuiteMissing(t *testing.T) {
	t.Parallel()

	base := Inspection{
		Path:          "/tmp/base.json",
		Kind:          ObjectKindArtifact,
		Type:          "firety.skill-routing-eval",
		SchemaVersion: "1",
		Provenance: NormalizeRecord(Record{
			CommandOrigin: "firety skill eval",
			Profile:       "generic",
			Backends:      []string{"generic"},
		}),
	}
	candidate := Inspection{
		Path:          "/tmp/candidate.json",
		Kind:          ObjectKindArtifact,
		Type:          "firety.skill-routing-eval",
		SchemaVersion: "1",
		Provenance: NormalizeRecord(Record{
			CommandOrigin: "firety skill eval",
			Profile:       "generic",
			SuitePath:     "evals/routing.json",
			Backends:      []string{"generic"},
		}),
	}

	result := Compare(base, candidate)
	if result.Status != ComparePartiallyComparable {
		t.Fatalf("expected partially comparable, got %#v", result)
	}
	if len(result.Reasons) == 0 {
		t.Fatalf("expected comparability reason, got %#v", result)
	}
}

func TestCompareNotComparableWhenBackendsDiffer(t *testing.T) {
	t.Parallel()

	base := Inspection{
		Path:          "/tmp/base.json",
		Kind:          ObjectKindArtifact,
		Type:          "firety.skill-routing-eval-multi",
		SchemaVersion: "1",
		Provenance: NormalizeRecord(Record{
			CommandOrigin: "firety skill eval",
			SuitePath:     "evals/routing.json",
			Backends:      []string{"codex", "generic"},
		}),
	}
	candidate := Inspection{
		Path:          "/tmp/candidate.json",
		Kind:          ObjectKindArtifact,
		Type:          "firety.skill-routing-eval-multi",
		SchemaVersion: "1",
		Provenance: NormalizeRecord(Record{
			CommandOrigin: "firety skill eval",
			SuitePath:     "evals/routing.json",
			Backends:      []string{"cursor", "generic"},
		}),
	}

	result := Compare(base, candidate)
	if result.Status != CompareNotComparable {
		t.Fatalf("expected not comparable, got %#v", result)
	}
	if result.RerunRecommendation == "" {
		t.Fatalf("expected rerun recommendation, got %#v", result)
	}
}
