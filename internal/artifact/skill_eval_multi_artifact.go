package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	domaineval "github.com/firety/firety/internal/domain/eval"
)

const SkillEvalMultiArtifactSchemaVersion = "1"

type SkillEvalMultiArtifactOptions struct {
	Format   string
	Suite    string
	Backends []domaineval.RoutingEvalBackendInfo
}

type SkillEvalMultiArtifact struct {
	SchemaVersion  string                                 `json:"schema_version"`
	ArtifactType   string                                 `json:"artifact_type"`
	Tool           SkillLintArtifactTool                  `json:"tool"`
	Run            SkillEvalMultiArtifactRun              `json:"run"`
	Suite          domaineval.RoutingEvalSuiteInfo        `json:"suite"`
	Backends       []domaineval.RoutingEvalBackendInfo    `json:"backends"`
	Results        []domaineval.BackendEvalReport         `json:"results"`
	Summary        domaineval.MultiBackendEvalSummary     `json:"summary"`
	DifferingCases []domaineval.MultiBackendDifferingCase `json:"differing_cases,omitempty"`
	Fingerprint    string                                 `json:"fingerprint,omitempty"`
}

type SkillEvalMultiArtifactRun struct {
	Target       string `json:"target"`
	SuitePath    string `json:"suite_path"`
	ExitCode     int    `json:"exit_code"`
	StdoutFormat string `json:"stdout_format"`
}

func BuildSkillEvalMultiArtifact(version app.VersionInfo, report domaineval.MultiBackendEvalReport, options SkillEvalMultiArtifactOptions, exitCode int) SkillEvalMultiArtifact {
	artifact := SkillEvalMultiArtifact{
		SchemaVersion: SkillEvalMultiArtifactSchemaVersion,
		ArtifactType:  "firety.skill-routing-eval-multi",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillEvalMultiArtifactRun{
			Target:       report.Target,
			SuitePath:    report.Suite.Path,
			ExitCode:     exitCode,
			StdoutFormat: evalMultiOptionsOutputFormatDefault(options),
		},
		Suite:          report.Suite,
		Backends:       collectMultiArtifactBackends(report.Backends),
		Results:        report.Backends,
		Summary:        report.Summary,
		DifferingCases: report.DifferingCases,
	}
	artifact.Fingerprint = skillEvalMultiArtifactFingerprint(artifact)
	return artifact
}

func WriteSkillEvalMultiArtifact(path string, artifact SkillEvalMultiArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillEvalMultiArtifact(artifact)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func marshalSkillEvalMultiArtifact(artifact SkillEvalMultiArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillEvalMultiArtifactFingerprint(artifact SkillEvalMultiArtifact) string {
	type fingerprintInput struct {
		SchemaVersion  string                                 `json:"schema_version"`
		Target         string                                 `json:"target"`
		SuitePath      string                                 `json:"suite_path"`
		Backends       []domaineval.RoutingEvalBackendInfo    `json:"backends"`
		Summary        domaineval.MultiBackendEvalSummary     `json:"summary"`
		DifferingCases []domaineval.MultiBackendDifferingCase `json:"differing_cases,omitempty"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion:  artifact.SchemaVersion,
		Target:         artifact.Run.Target,
		SuitePath:      artifact.Run.SuitePath,
		Backends:       artifact.Backends,
		Summary:        artifact.Summary,
		DifferingCases: artifact.DifferingCases,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func collectMultiArtifactBackends(reports []domaineval.BackendEvalReport) []domaineval.RoutingEvalBackendInfo {
	backends := make([]domaineval.RoutingEvalBackendInfo, 0, len(reports))
	for _, report := range reports {
		backends = append(backends, report.Backend)
	}
	return backends
}

func evalMultiOptionsOutputFormatDefault(options SkillEvalMultiArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
