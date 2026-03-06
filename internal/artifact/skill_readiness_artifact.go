package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/readiness"
)

const SkillReadinessArtifactSchemaVersion = "1"

type SkillReadinessArtifactOptions struct {
	Format         string
	Target         string
	PublishContext string
	Profiles       []string
	Strictness     string
	SuitePath      string
	Runner         string
	Backends       []string
	InputArtifacts []string
	InputPacks     []string
	InputReports   []string
}

type SkillReadinessArtifact struct {
	SchemaVersion string                    `json:"schema_version"`
	ArtifactType  string                    `json:"artifact_type"`
	Tool          SkillLintArtifactTool     `json:"tool"`
	Run           SkillReadinessArtifactRun `json:"run"`
	Readiness     readiness.Result          `json:"readiness"`
	Fingerprint   string                    `json:"fingerprint,omitempty"`
}

type SkillReadinessArtifactRun struct {
	Target         string   `json:"target,omitempty"`
	PublishContext string   `json:"publish_context"`
	Profiles       []string `json:"profiles,omitempty"`
	Strictness     string   `json:"strictness,omitempty"`
	SuitePath      string   `json:"suite_path,omitempty"`
	Runner         string   `json:"runner,omitempty"`
	Backends       []string `json:"backends,omitempty"`
	InputArtifacts []string `json:"input_artifacts,omitempty"`
	InputPacks     []string `json:"input_packs,omitempty"`
	InputReports   []string `json:"input_reports,omitempty"`
	ExitCode       int      `json:"exit_code"`
	StdoutFormat   string   `json:"stdout_format"`
}

func BuildSkillReadinessArtifact(version app.VersionInfo, result readiness.Result, options SkillReadinessArtifactOptions, exitCode int) SkillReadinessArtifact {
	value := SkillReadinessArtifact{
		SchemaVersion: SkillReadinessArtifactSchemaVersion,
		ArtifactType:  "firety.skill-readiness",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillReadinessArtifactRun{
			Target:         options.Target,
			PublishContext: options.PublishContext,
			Profiles:       append([]string(nil), options.Profiles...),
			Strictness:     options.Strictness,
			SuitePath:      options.SuitePath,
			Runner:         options.Runner,
			Backends:       append([]string(nil), options.Backends...),
			InputArtifacts: append([]string(nil), options.InputArtifacts...),
			InputPacks:     append([]string(nil), options.InputPacks...),
			InputReports:   append([]string(nil), options.InputReports...),
			ExitCode:       exitCode,
			StdoutFormat:   readinessOptionsOutputFormatDefault(options),
		},
		Readiness: result,
	}
	value.Fingerprint = skillReadinessArtifactFingerprint(value)
	return value
}

func WriteSkillReadinessArtifact(path string, value SkillReadinessArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func skillReadinessArtifactFingerprint(value SkillReadinessArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                    `json:"schema_version"`
		Run           SkillReadinessArtifactRun `json:"run"`
		Readiness     readiness.Result          `json:"readiness"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: value.SchemaVersion,
		Run:           value.Run,
		Readiness:     value.Readiness,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func readinessOptionsOutputFormatDefault(options SkillReadinessArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
