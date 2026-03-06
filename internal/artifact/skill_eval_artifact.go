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

const SkillEvalArtifactSchemaVersion = "1"

type SkillEvalArtifactOptions struct {
	Format  string
	Profile string
	Suite   string
	Runner  string
}

type SkillEvalArtifact struct {
	SchemaVersion string                             `json:"schema_version"`
	ArtifactType  string                             `json:"artifact_type"`
	Tool          SkillLintArtifactTool              `json:"tool"`
	Run           SkillEvalArtifactRun               `json:"run"`
	Suite         domaineval.RoutingEvalSuiteInfo    `json:"suite"`
	Backend       domaineval.RoutingEvalBackendInfo  `json:"backend"`
	Summary       domaineval.RoutingEvalSummary      `json:"summary"`
	Results       []domaineval.RoutingEvalCaseResult `json:"results"`
	Fingerprint   string                             `json:"fingerprint,omitempty"`
}

type SkillEvalArtifactRun struct {
	Target       string `json:"target"`
	Profile      string `json:"profile"`
	SuitePath    string `json:"suite_path"`
	Runner       string `json:"runner,omitempty"`
	ExitCode     int    `json:"exit_code"`
	StdoutFormat string `json:"stdout_format"`
}

func BuildSkillEvalArtifact(version app.VersionInfo, report domaineval.RoutingEvalReport, options SkillEvalArtifactOptions, exitCode int) SkillEvalArtifact {
	artifact := SkillEvalArtifact{
		SchemaVersion: SkillEvalArtifactSchemaVersion,
		ArtifactType:  "firety.skill-routing-eval",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillEvalArtifactRun{
			Target:       report.Target,
			Profile:      options.Profile,
			SuitePath:    report.Suite.Path,
			Runner:       options.Runner,
			ExitCode:     exitCode,
			StdoutFormat: evalOptionsOutputFormatDefault(options),
		},
		Suite:   report.Suite,
		Backend: report.Backend,
		Summary: report.Summary,
		Results: report.Results,
	}
	artifact.Fingerprint = skillEvalArtifactFingerprint(artifact)

	return artifact
}

func WriteSkillEvalArtifact(path string, artifact SkillEvalArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillEvalArtifact(artifact)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func marshalSkillEvalArtifact(artifact SkillEvalArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillEvalArtifactFingerprint(artifact SkillEvalArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                             `json:"schema_version"`
		Target        string                             `json:"target"`
		Profile       string                             `json:"profile"`
		SuitePath     string                             `json:"suite_path"`
		Backend       string                             `json:"backend"`
		Summary       domaineval.RoutingEvalSummary      `json:"summary"`
		Results       []domaineval.RoutingEvalCaseResult `json:"results"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		Target:        artifact.Run.Target,
		Profile:       artifact.Run.Profile,
		SuitePath:     artifact.Run.SuitePath,
		Backend:       artifact.Backend.Name,
		Summary:       artifact.Summary,
		Results:       artifact.Results,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func evalOptionsOutputFormatDefault(options SkillEvalArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
