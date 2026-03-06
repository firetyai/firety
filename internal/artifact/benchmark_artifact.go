package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/benchmark"
)

const BenchmarkArtifactSchemaVersion = "1"

type BenchmarkArtifactOptions struct {
	Format string
}

type BenchmarkArtifact struct {
	SchemaVersion string                      `json:"schema_version"`
	ArtifactType  string                      `json:"artifact_type"`
	Tool          SkillLintArtifactTool       `json:"tool"`
	Run           BenchmarkArtifactRun        `json:"run"`
	Suite         benchmark.SuiteInfo         `json:"suite"`
	Summary       benchmark.Summary           `json:"summary"`
	Categories    []benchmark.CategorySummary `json:"categories"`
	Fixtures      []benchmark.FixtureResult   `json:"fixtures"`
	Fingerprint   string                      `json:"fingerprint,omitempty"`
}

type BenchmarkArtifactRun struct {
	ExitCode     int    `json:"exit_code"`
	StdoutFormat string `json:"stdout_format"`
}

func BuildBenchmarkArtifact(version app.VersionInfo, report benchmark.Report, options BenchmarkArtifactOptions, exitCode int) BenchmarkArtifact {
	artifact := BenchmarkArtifact{
		SchemaVersion: BenchmarkArtifactSchemaVersion,
		ArtifactType:  "firety.benchmark-report",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: BenchmarkArtifactRun{
			ExitCode:     exitCode,
			StdoutFormat: benchmarkArtifactOutputFormatDefault(options),
		},
		Suite:      report.Suite,
		Summary:    report.Summary,
		Categories: report.Categories,
		Fixtures:   report.Fixtures,
	}
	artifact.Fingerprint = benchmarkArtifactFingerprint(artifact)
	return artifact
}

func WriteBenchmarkArtifact(path string, artifact BenchmarkArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalBenchmarkArtifact(artifact)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func marshalBenchmarkArtifact(artifact BenchmarkArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func benchmarkArtifactFingerprint(artifact BenchmarkArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                      `json:"schema_version"`
		Suite         benchmark.SuiteInfo         `json:"suite"`
		Summary       benchmark.Summary           `json:"summary"`
		Categories    []benchmark.CategorySummary `json:"categories"`
		Fixtures      []benchmark.FixtureResult   `json:"fixtures"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		Suite:         artifact.Suite,
		Summary:       artifact.Summary,
		Categories:    artifact.Categories,
		Fixtures:      artifact.Fixtures,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func benchmarkArtifactOutputFormatDefault(options BenchmarkArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
