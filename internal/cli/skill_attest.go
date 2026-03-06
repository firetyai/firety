package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/artifact"
	"github.com/firety/firety/internal/domain/attestation"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
	"github.com/spf13/cobra"
)

const skillAttestationJSONSchemaVersion = "1"

type skillAttestOptions struct {
	format         string
	profiles       []string
	strictness     string
	suite          string
	runner         string
	backends       []string
	inputArtifacts []string
	inputPacks     []string
	includeGate    bool
	artifact       string
}

func newSkillAttestCommand(application *app.App) *cobra.Command {
	options := skillAttestOptions{
		format:     skillLintFormatText,
		strictness: string(lint.StrictnessDefault),
	}

	cmd := &cobra.Command{
		Use:   "attest [path]",
		Short: "Generate an evidence-backed support attestation",
		Long:  "Generate a publishable Firety support-claims manifest that states what a skill supports, what was tested, what gate evidence exists, and what limitations remain.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(args); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			target := ""
			if len(args) == 1 {
				target = args[0]
			}

			profiles, err := service.ParseSkillLintProfiles(options.profiles)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			strictness, err := lint.ParseStrictness(options.strictness)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			backends, err := parseSkillEvalBackendSelections(options.backends)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}
			inputArtifacts, err := resolveAttestationInputArtifacts(options.inputArtifacts, options.inputPacks)
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			result, err := application.Services.SkillAttest.Generate(target, service.SkillAttestOptions{
				Profiles:       profiles,
				Strictness:     strictness,
				SuitePath:      options.suite,
				Runner:         options.runner,
				Backends:       backends,
				InputArtifacts: inputArtifacts,
				IncludeGate:    options.includeGate,
			})
			if err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if err := writeSkillAttestationReport(cmd.OutOrStdout(), result.Report, options); err != nil {
				return newExitError(ExitCodeRuntime, err)
			}

			if options.artifact != "" {
				value := artifact.BuildSkillAttestationArtifact(application.Version, result.Report, artifact.SkillAttestationArtifactOptions{
					Format:         options.format,
					Target:         target,
					Profiles:       append([]string(nil), options.profiles...),
					Strictness:     strictness.DisplayName(),
					SuitePath:      options.suite,
					Backends:       append([]string(nil), options.backends...),
					InputArtifacts: append([]string(nil), inputArtifacts...),
					InputPacks:     append([]string(nil), options.inputPacks...),
					IncludeGate:    options.includeGate,
				})
				if err := artifact.WriteSkillAttestationArtifact(options.artifact, value); err != nil {
					return newExitError(ExitCodeRuntime, err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&options.format, "format", skillLintFormatText, "Output format: text or json")
	cmd.Flags().StringArrayVar(&options.profiles, "profile", nil, "Profile to evaluate for fresh attestation generation; repeat for multiple profiles")
	cmd.Flags().StringVar(&options.strictness, "strictness", string(lint.StrictnessDefault), "Lint strictness for fresh attestation generation")
	cmd.Flags().StringVar(&options.suite, "suite", "", "Routing eval suite path for fresh measured evidence")
	cmd.Flags().StringVar(&options.runner, "runner", "", "Single-backend routing eval runner for fresh measured evidence")
	cmd.Flags().StringArrayVar(&options.backends, "backend", nil, `Backend selection in the form "<id>" or "<id>=/path/to/runner"; repeat for multiple backends`)
	cmd.Flags().StringArrayVar(&options.inputArtifacts, "input-artifact", nil, "Use existing Firety artifacts instead of rerunning analysis")
	cmd.Flags().StringArrayVar(&options.inputPacks, "input-pack", nil, "Use one or more existing Firety evidence-pack directories")
	cmd.Flags().BoolVar(&options.includeGate, "include-gate", false, "Include a quality-gate summary when fresh evidence or artifact evidence supports it")
	cmd.Flags().StringVar(&options.artifact, "artifact", "", "Write a versioned machine-readable attestation artifact to the given file path")

	return cmd
}

func (o skillAttestOptions) Validate(args []string) error {
	switch o.format {
	case skillLintFormatText, skillLintFormatJSON:
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", o.format, skillLintFormatText, skillLintFormatJSON)
	}
	if o.artifact == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	if _, err := lint.ParseStrictness(o.strictness); err != nil {
		return err
	}
	if _, err := service.ParseSkillLintProfiles(o.profiles); err != nil {
		return err
	}
	if len(o.inputArtifacts) > 0 || len(o.inputPacks) > 0 {
		if len(args) > 0 {
			return fmt.Errorf("artifact-based attestation cannot be combined with a target path")
		}
		if len(o.profiles) > 0 || o.strictness != string(lint.StrictnessDefault) || o.suite != "" || o.runner != "" || len(o.backends) > 0 {
			return fmt.Errorf("artifact-based attestation cannot be combined with fresh-run profile, strictness, suite, runner, or backend flags")
		}
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("attestation requires a target path or at least one --input-artifact/--input-pack value")
	}
	if len(o.backends) > 0 && o.runner != "" {
		return fmt.Errorf("--runner cannot be combined with --backend")
	}
	return nil
}

func resolveAttestationInputArtifacts(inputs, packs []string) ([]string, error) {
	paths := append([]string(nil), inputs...)
	for _, pack := range packs {
		packPaths, err := loadAttestationPackArtifacts(pack)
		if err != nil {
			return nil, err
		}
		paths = append(paths, packPaths...)
	}
	return paths, nil
}

func loadAttestationPackArtifacts(packDir string) ([]string, error) {
	manifestPath := filepath.Join(packDir, "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest struct {
		PackType  string `json:"pack_type"`
		Artifacts []struct {
			Path string `json:"path"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("parse evidence-pack manifest: %w", err)
	}
	if manifest.PackType != "firety.evidence-pack" {
		return nil, fmt.Errorf("directory %s does not contain a supported Firety evidence pack", packDir)
	}

	out := make([]string, 0, len(manifest.Artifacts))
	for _, item := range manifest.Artifacts {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		out = append(out, filepath.Join(packDir, filepath.FromSlash(item.Path)))
	}
	return out, nil
}

func writeSkillAttestationReport(w io.Writer, report attestation.Report, options skillAttestOptions) error {
	switch options.format {
	case skillLintFormatText:
		return writeSkillAttestationText(w, report)
	case skillLintFormatJSON:
		payload := struct {
			SchemaVersion string             `json:"schema_version"`
			Report        attestation.Report `json:"report"`
		}{
			SchemaVersion: skillAttestationJSONSchemaVersion,
			Report:        report,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	default:
		return fmt.Errorf("invalid format %q: must be one of %s, %s", options.format, skillLintFormatText, skillLintFormatJSON)
	}
}

func writeSkillAttestationText(w io.Writer, report attestation.Report) error {
	if report.Target != "" {
		if _, err := fmt.Fprintf(w, "Target: %s\n", report.Target); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Support posture: %s\n", strings.ToUpper(string(report.SupportPosture))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Evidence: %s\n", strings.ToUpper(string(report.EvidenceLevel))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Summary: %s\n", report.Summary); err != nil {
		return err
	}
	if len(report.SupportedProfiles) > 0 {
		if _, err := fmt.Fprintf(w, "Supported profiles: %s\n", strings.Join(report.SupportedProfiles, ", ")); err != nil {
			return err
		}
	}
	if len(report.TestedProfiles) > 0 {
		if _, err := fmt.Fprintf(w, "Tested profiles: %s\n", strings.Join(report.TestedProfiles, ", ")); err != nil {
			return err
		}
	}
	if len(report.SupportedBackends) > 0 {
		if _, err := fmt.Fprintf(w, "Supported backends: %s\n", strings.Join(report.SupportedBackends, ", ")); err != nil {
			return err
		}
	}
	if len(report.TestedBackends) > 0 {
		if _, err := fmt.Fprintln(w, "Tested backends:"); err != nil {
			return err
		}
		for _, backend := range report.TestedBackends {
			if _, err := fmt.Fprintf(w, "- %s: %s\n", backend.BackendName, backend.Summary); err != nil {
				return err
			}
		}
	}
	if report.QualityGate != nil {
		if _, err := fmt.Fprintf(w, "Quality gate: %s (%s)\n", strings.ToUpper(report.QualityGate.Decision), report.QualityGate.Summary); err != nil {
			return err
		}
	}
	if len(report.Claims) > 0 {
		if _, err := fmt.Fprintln(w, "Claims:"); err != nil {
			return err
		}
		for _, claim := range report.Claims {
			if _, err := fmt.Fprintf(w, "- %s\n", claim.Statement); err != nil {
				return err
			}
		}
	}
	if len(report.Limitations) > 0 {
		if _, err := fmt.Fprintln(w, "Known limitations:"); err != nil {
			return err
		}
		for _, item := range report.Limitations {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(report.RecommendedConsumerReadingOrder) > 0 {
		if _, err := fmt.Fprintln(w, "Read next:"); err != nil {
			return err
		}
		for _, item := range report.RecommendedConsumerReadingOrder {
			if _, err := fmt.Fprintf(w, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	return nil
}
