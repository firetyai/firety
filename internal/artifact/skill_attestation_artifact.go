package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/attestation"
)

const SkillAttestationArtifactSchemaVersion = "1"

type SkillAttestationArtifactOptions struct {
	Format         string
	Target         string
	Profiles       []string
	Strictness     string
	SuitePath      string
	Backends       []string
	InputArtifacts []string
	InputPacks     []string
	IncludeGate    bool
}

type SkillAttestationArtifact struct {
	SchemaVersion string                      `json:"schema_version"`
	ArtifactType  string                      `json:"artifact_type"`
	Tool          SkillLintArtifactTool       `json:"tool"`
	Run           SkillAttestationArtifactRun `json:"run"`
	Attestation   attestation.Report          `json:"attestation"`
	Fingerprint   string                      `json:"fingerprint,omitempty"`
}

type SkillAttestationArtifactRun struct {
	Target         string   `json:"target,omitempty"`
	Profiles       []string `json:"profiles,omitempty"`
	Strictness     string   `json:"strictness,omitempty"`
	SuitePath      string   `json:"suite_path,omitempty"`
	Backends       []string `json:"backends,omitempty"`
	InputArtifacts []string `json:"input_artifacts,omitempty"`
	InputPacks     []string `json:"input_packs,omitempty"`
	IncludeGate    bool     `json:"include_gate,omitempty"`
	StdoutFormat   string   `json:"stdout_format"`
}

func BuildSkillAttestationArtifact(version app.VersionInfo, report attestation.Report, options SkillAttestationArtifactOptions) SkillAttestationArtifact {
	value := SkillAttestationArtifact{
		SchemaVersion: SkillAttestationArtifactSchemaVersion,
		ArtifactType:  "firety.skill-attestation",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillAttestationArtifactRun{
			Target:         options.Target,
			Profiles:       append([]string(nil), options.Profiles...),
			Strictness:     options.Strictness,
			SuitePath:      options.SuitePath,
			Backends:       append([]string(nil), options.Backends...),
			InputArtifacts: append([]string(nil), options.InputArtifacts...),
			InputPacks:     append([]string(nil), options.InputPacks...),
			IncludeGate:    options.IncludeGate,
			StdoutFormat:   attestationOptionsOutputFormatDefault(options),
		},
		Attestation: report,
	}
	value.Fingerprint = skillAttestationArtifactFingerprint(value)
	return value
}

func WriteSkillAttestationArtifact(path string, value SkillAttestationArtifact) error {
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

func skillAttestationArtifactFingerprint(value SkillAttestationArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                      `json:"schema_version"`
		Run           SkillAttestationArtifactRun `json:"run"`
		Attestation   attestation.Report          `json:"attestation"`
	}
	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: value.SchemaVersion,
		Run:           value.Run,
		Attestation:   value.Attestation,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func attestationOptionsOutputFormatDefault(options SkillAttestationArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
