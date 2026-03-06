package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/gate"
)

const SkillGateArtifactSchemaVersion = "1"

type SkillGateArtifactOptions struct {
	Format         string
	Target         string
	BaseTarget     string
	BaselinePath   string
	Profile        string
	Strictness     string
	SuitePath      string
	Runner         string
	Backends       []string
	InputArtifacts []string
}

type SkillGateArtifact struct {
	SchemaVersion string                `json:"schema_version"`
	ArtifactType  string                `json:"artifact_type"`
	Tool          SkillLintArtifactTool `json:"tool"`
	Run           SkillGateArtifactRun  `json:"run"`
	Result        gate.Result           `json:"result"`
	Fingerprint   string                `json:"fingerprint,omitempty"`
}

type SkillGateArtifactRun struct {
	Target         string   `json:"target,omitempty"`
	BaseTarget     string   `json:"base_target,omitempty"`
	BaselinePath   string   `json:"baseline_path,omitempty"`
	Profile        string   `json:"profile"`
	Strictness     string   `json:"strictness"`
	SuitePath      string   `json:"suite_path,omitempty"`
	Runner         string   `json:"runner,omitempty"`
	Backends       []string `json:"backends,omitempty"`
	InputArtifacts []string `json:"input_artifacts,omitempty"`
	ExitCode       int      `json:"exit_code"`
	StdoutFormat   string   `json:"stdout_format"`
}

func BuildSkillGateArtifact(version app.VersionInfo, result gate.Result, options SkillGateArtifactOptions, exitCode int) SkillGateArtifact {
	artifact := SkillGateArtifact{
		SchemaVersion: SkillGateArtifactSchemaVersion,
		ArtifactType:  "firety.skill-quality-gate",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillGateArtifactRun{
			Target:         options.Target,
			BaseTarget:     options.BaseTarget,
			BaselinePath:   options.BaselinePath,
			Profile:        options.Profile,
			Strictness:     options.Strictness,
			SuitePath:      options.SuitePath,
			Runner:         options.Runner,
			Backends:       append([]string(nil), options.Backends...),
			InputArtifacts: append([]string(nil), options.InputArtifacts...),
			ExitCode:       exitCode,
			StdoutFormat:   gateOptionsOutputFormatDefault(options),
		},
		Result: result,
	}
	artifact.Fingerprint = skillGateArtifactFingerprint(artifact)
	return artifact
}

func WriteSkillGateArtifact(path string, artifact SkillGateArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillGateArtifact(artifact)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func marshalSkillGateArtifact(artifact SkillGateArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillGateArtifactFingerprint(artifact SkillGateArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string      `json:"schema_version"`
		Target        string      `json:"target,omitempty"`
		BaseTarget    string      `json:"base_target,omitempty"`
		BaselinePath  string      `json:"baseline_path,omitempty"`
		Profile       string      `json:"profile"`
		Strictness    string      `json:"strictness"`
		SuitePath     string      `json:"suite_path,omitempty"`
		Backends      []string    `json:"backends,omitempty"`
		Result        gate.Result `json:"result"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		Target:        artifact.Run.Target,
		BaseTarget:    artifact.Run.BaseTarget,
		BaselinePath:  artifact.Run.BaselinePath,
		Profile:       artifact.Run.Profile,
		Strictness:    artifact.Run.Strictness,
		SuitePath:     artifact.Run.SuitePath,
		Backends:      artifact.Run.Backends,
		Result:        artifact.Result,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func gateOptionsOutputFormatDefault(options SkillGateArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
