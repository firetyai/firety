package lint_test

import (
	"testing"

	"github.com/firety/firety/internal/domain/lint"
)

func TestCompareReportsDetectsSeverityChanges(t *testing.T) {
	t.Parallel()

	base := lint.Report{
		Target: "base",
		Findings: []lint.Finding{
			{
				RuleID:   lint.RuleMissingExamples.ID,
				Severity: lint.SeverityWarning,
				Path:     "SKILL.md",
				Message:  "no obvious examples section found",
				Line:     12,
			},
		},
	}
	candidate := lint.Report{
		Target: "candidate",
		Findings: []lint.Finding{
			{
				RuleID:   lint.RuleMissingExamples.ID,
				Severity: lint.SeverityError,
				Path:     "SKILL.md",
				Message:  "no obvious examples section found",
				Line:     14,
			},
		},
	}

	comparison := lint.CompareReports(base, candidate)
	if comparison.Summary.SeverityChangedCount != 1 {
		t.Fatalf("expected one severity change, got %#v", comparison)
	}
	if len(comparison.ChangedFindings) != 1 {
		t.Fatalf("expected changed finding, got %#v", comparison.ChangedFindings)
	}
	if comparison.ChangedFindings[0].BaseSeverity != lint.SeverityWarning {
		t.Fatalf("expected base severity warning, got %#v", comparison.ChangedFindings[0])
	}
	if comparison.ChangedFindings[0].CandidateSeverity != lint.SeverityError {
		t.Fatalf("expected candidate severity error, got %#v", comparison.ChangedFindings[0])
	}
	if comparison.ChangedFindings[0].BaseLine == nil || *comparison.ChangedFindings[0].BaseLine != 12 {
		t.Fatalf("expected base line 12, got %#v", comparison.ChangedFindings[0])
	}
	if comparison.ChangedFindings[0].CandidateLine == nil || *comparison.ChangedFindings[0].CandidateLine != 14 {
		t.Fatalf("expected candidate line 14, got %#v", comparison.ChangedFindings[0])
	}
}

func TestCompareReportsDeterministicOrdering(t *testing.T) {
	t.Parallel()

	base := lint.Report{
		Target: "base",
		Findings: []lint.Finding{
			{
				RuleID:   lint.RuleMissingTitle.ID,
				Severity: lint.SeverityError,
				Path:     "SKILL.md",
				Message:  "missing title",
			},
		},
	}
	candidate := lint.Report{
		Target: "candidate",
		Findings: []lint.Finding{
			{
				RuleID:   lint.RuleMissingExamples.ID,
				Severity: lint.SeverityWarning,
				Path:     "SKILL.md",
				Message:  "missing examples",
			},
			{
				RuleID:   lint.RuleMissingWhenToUse.ID,
				Severity: lint.SeverityWarning,
				Path:     "SKILL.md",
				Message:  "missing when to use",
			},
		},
	}

	first := lint.CompareReports(base, candidate)
	second := lint.CompareReports(base, candidate)

	if len(first.AddedFindings) != len(second.AddedFindings) || len(first.RemovedFindings) != len(second.RemovedFindings) {
		t.Fatalf("expected stable comparison sizes, first=%#v second=%#v", first, second)
	}
	for index := range first.AddedFindings {
		if first.AddedFindings[index] != second.AddedFindings[index] {
			t.Fatalf("expected deterministic added findings, first=%#v second=%#v", first.AddedFindings, second.AddedFindings)
		}
	}
	for index := range first.RemovedFindings {
		if first.RemovedFindings[index] != second.RemovedFindings[index] {
			t.Fatalf("expected deterministic removed findings, first=%#v second=%#v", first.RemovedFindings, second.RemovedFindings)
		}
	}
}
