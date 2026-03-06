package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const SchemaVersion = "1"

type ObjectKind string
type CompareStatus string

const (
	ObjectKindArtifact     ObjectKind = "artifact"
	ObjectKindEvidencePack ObjectKind = "evidence-pack"
	ObjectKindTrustReport  ObjectKind = "trust-report"

	CompareComparable          CompareStatus = "comparable"
	ComparePartiallyComparable CompareStatus = "partially-comparable"
	CompareNotComparable       CompareStatus = "not-comparable"
)

type Record struct {
	SchemaVersion        string   `json:"schema_version"`
	CommandOrigin        string   `json:"command_origin,omitempty"`
	FiretyVersion        string   `json:"firety_version,omitempty"`
	FiretyCommit         string   `json:"firety_commit,omitempty"`
	Target               string   `json:"target,omitempty"`
	TargetFingerprint    string   `json:"target_fingerprint,omitempty"`
	Profile              string   `json:"profile,omitempty"`
	Strictness           string   `json:"strictness,omitempty"`
	FailOn               string   `json:"fail_on,omitempty"`
	Explain              bool     `json:"explain,omitempty"`
	RoutingRisk          bool     `json:"routing_risk,omitempty"`
	SuitePath            string   `json:"suite_path,omitempty"`
	Backends             []string `json:"backends,omitempty"`
	InputArtifacts       []string `json:"input_artifacts,omitempty"`
	InputPacks           []string `json:"input_packs,omitempty"`
	ArtifactDependencies []string `json:"artifact_dependencies,omitempty"`
	ComparableKey        string   `json:"comparable_key,omitempty"`
	ComparabilityNotes   []string `json:"comparability_notes,omitempty"`
	ReproducibilityNotes []string `json:"reproducibility_notes,omitempty"`
}

type Inspection struct {
	Path                  string     `json:"path"`
	Kind                  ObjectKind `json:"kind"`
	Type                  string     `json:"type"`
	SchemaVersion         string     `json:"schema_version"`
	Summary               string     `json:"summary,omitempty"`
	Provenance            Record     `json:"provenance"`
	SuitableForBaseline   bool       `json:"suitable_for_baseline"`
	SuitableForComparison bool       `json:"suitable_for_comparison"`
}

type Comparison struct {
	SchemaVersion       string        `json:"schema_version"`
	BasePath            string        `json:"base_path"`
	CandidatePath       string        `json:"candidate_path"`
	Status              CompareStatus `json:"status"`
	Summary             string        `json:"summary"`
	Reasons             []string      `json:"reasons,omitempty"`
	SharedContext       []string      `json:"shared_context,omitempty"`
	RerunRecommendation string        `json:"rerun_recommendation,omitempty"`
}

func NewRecord() Record {
	return Record{SchemaVersion: SchemaVersion}
}

func NormalizeRecord(record Record) Record {
	record.SchemaVersion = SchemaVersion
	record.Backends = uniqueSorted(record.Backends)
	record.InputArtifacts = uniqueSorted(record.InputArtifacts)
	record.InputPacks = uniqueSorted(record.InputPacks)
	record.ArtifactDependencies = uniqueSorted(record.ArtifactDependencies)
	record.ComparabilityNotes = uniqueSorted(record.ComparabilityNotes)
	record.ReproducibilityNotes = uniqueSorted(record.ReproducibilityNotes)
	return record
}

func Compare(base, candidate Inspection) Comparison {
	result := Comparison{
		SchemaVersion: SchemaVersion,
		BasePath:      base.Path,
		CandidatePath: candidate.Path,
		Status:        CompareComparable,
	}

	reasons := make([]string, 0, 8)
	shared := make([]string, 0, 8)

	if base.Kind != candidate.Kind {
		reasons = append(reasons, fmt.Sprintf("not comparable because object kinds differ: %s vs %s", base.Kind, candidate.Kind))
		result.Status = CompareNotComparable
	}
	if base.Type != candidate.Type {
		reasons = append(reasons, fmt.Sprintf("not comparable because object types differ: %s vs %s", base.Type, candidate.Type))
		result.Status = CompareNotComparable
	}

	if result.Status != CompareNotComparable {
		compareField(&result, "profile", base.Provenance.Profile, candidate.Provenance.Profile, false, &reasons, &shared)
		compareField(&result, "strictness", base.Provenance.Strictness, candidate.Provenance.Strictness, false, &reasons, &shared)
		compareField(&result, "suite", base.Provenance.SuitePath, candidate.Provenance.SuitePath, suiteRequired(base, candidate), &reasons, &shared)
		compareBackends(&result, base.Provenance.Backends, candidate.Provenance.Backends, backendSetRequired(base, candidate), &reasons, &shared)
		compareField(&result, "command origin", base.Provenance.CommandOrigin, candidate.Provenance.CommandOrigin, false, &reasons, &shared)
	}

	if base.Provenance.TargetFingerprint != "" && candidate.Provenance.TargetFingerprint != "" {
		if base.Provenance.TargetFingerprint != candidate.Provenance.TargetFingerprint {
			reasons = append(reasons, "target fingerprints differ, indicating different content snapshots")
		} else {
			shared = append(shared, "same target fingerprint")
		}
	}

	if result.Status == CompareComparable && (len(base.Provenance.ComparabilityNotes) > 0 || len(candidate.Provenance.ComparabilityNotes) > 0) {
		reasons = append(reasons, append([]string{}, base.Provenance.ComparabilityNotes...)...)
		reasons = append(reasons, append([]string{}, candidate.Provenance.ComparabilityNotes...)...)
		result.Status = ComparePartiallyComparable
	}

	reasons = uniqueSorted(reasons)
	shared = uniqueSorted(shared)
	result.Reasons = reasons
	result.SharedContext = shared
	result.Summary = summarizeComparison(result.Status, reasons, shared)
	result.RerunRecommendation = rerunRecommendation(result.Status, reasons)
	return result
}

func FingerprintDirectory(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	type entry struct {
		Path string `json:"path"`
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	}
	entries := make([]entry, 0)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		entries = append(entries, entry{
			Path: filepath.ToSlash(rel),
			Hash: hex.EncodeToString(sum[:]),
			Size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return "", err
	}

	slices.SortFunc(entries, func(left, right entry) int {
		return strings.Compare(left.Path, right.Path)
	})
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func compareField(result *Comparison, label, base, candidate string, required bool, reasons, shared *[]string) {
	base = strings.TrimSpace(base)
	candidate = strings.TrimSpace(candidate)
	switch {
	case base == "" && candidate == "":
		if required {
			*reasons = append(*reasons, fmt.Sprintf("%s provenance is missing on both sides", label))
			if result.Status == CompareComparable {
				result.Status = ComparePartiallyComparable
			}
		}
	case base == "" || candidate == "":
		*reasons = append(*reasons, fmt.Sprintf("%s provenance is incomplete", label))
		if result.Status == CompareComparable {
			result.Status = ComparePartiallyComparable
		}
	case base != candidate:
		*reasons = append(*reasons, fmt.Sprintf("not comparable because %s differs: %s vs %s", label, base, candidate))
		result.Status = CompareNotComparable
	default:
		*shared = append(*shared, fmt.Sprintf("%s %s", label, base))
	}
}

func compareBackends(result *Comparison, base, candidate []string, required bool, reasons, shared *[]string) {
	base = uniqueSorted(base)
	candidate = uniqueSorted(candidate)
	switch {
	case len(base) == 0 && len(candidate) == 0:
		if required {
			*reasons = append(*reasons, "backend provenance is missing on both sides")
			if result.Status == CompareComparable {
				result.Status = ComparePartiallyComparable
			}
		}
	case len(base) == 0 || len(candidate) == 0:
		*reasons = append(*reasons, "backend provenance is incomplete")
		if result.Status == CompareComparable {
			result.Status = ComparePartiallyComparable
		}
	case !slices.Equal(base, candidate):
		*reasons = append(*reasons, fmt.Sprintf("not comparable because backend sets differ: %s vs %s", strings.Join(base, ", "), strings.Join(candidate, ", ")))
		result.Status = CompareNotComparable
	default:
		*shared = append(*shared, fmt.Sprintf("backends %s", strings.Join(base, ", ")))
	}
}

func suiteRequired(base, candidate Inspection) bool {
	return strings.Contains(base.Type, "eval") || strings.Contains(candidate.Type, "eval")
}

func backendSetRequired(base, candidate Inspection) bool {
	return strings.Contains(base.Type, "multi") || strings.Contains(candidate.Type, "multi")
}

func summarizeComparison(status CompareStatus, reasons, shared []string) string {
	switch status {
	case CompareComparable:
		if len(shared) > 0 {
			return fmt.Sprintf("Comparable: %s.", strings.Join(shared, "; "))
		}
		return "Comparable based on the captured provenance."
	case ComparePartiallyComparable:
		if len(reasons) > 0 {
			return fmt.Sprintf("Partially comparable with caveats: %s.", strings.Join(firstN(reasons, 2), "; "))
		}
		return "Partially comparable with provenance caveats."
	default:
		if len(reasons) > 0 {
			return fmt.Sprintf("Not comparable: %s.", strings.Join(firstN(reasons, 2), "; "))
		}
		return "Not comparable based on the captured provenance."
	}
}

func rerunRecommendation(status CompareStatus, reasons []string) string {
	switch status {
	case CompareComparable:
		return "Existing outputs are comparable; a rerun is optional."
	case ComparePartiallyComparable:
		return "Review the provenance caveats before relying on a direct comparison; rerun with matched options if the distinction matters."
	default:
		return "Rerun the analysis with matched profiles, strictness, suites, and backend sets instead of comparing these outputs directly."
	}
}

func uniqueSorted(values []string) []string {
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

func firstN(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
