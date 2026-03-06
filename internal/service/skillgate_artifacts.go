package service

import (
	domaineval "github.com/firety/firety/internal/domain/eval"
	domaingate "github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
)

type gateArtifactEnvelope struct {
	ArtifactType string `json:"artifact_type"`
}

type gateSkillLintArtifact struct {
	Run struct {
		Target     string `json:"target"`
		Profile    string `json:"profile"`
		Strictness string `json:"strictness"`
	} `json:"run"`
	Summary struct {
		ErrorCount   int `json:"error_count"`
		WarningCount int `json:"warning_count"`
	} `json:"summary"`
	Findings    []gateSkillLintArtifactFinding `json:"findings"`
	RoutingRisk *lint.RoutingRiskSummary       `json:"routing_risk,omitempty"`
}

type gateSkillLintArtifactFinding struct {
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Line     *int   `json:"line,omitempty"`
	Message  string `json:"message"`
}

type gateSkillAnalysisArtifact struct {
	Run struct {
		Target     string `json:"target"`
		Profile    string `json:"profile"`
		Strictness string `json:"strictness"`
		Runner     string `json:"runner,omitempty"`
	} `json:"run"`
	Lint struct {
		Summary struct {
			ErrorCount   int `json:"error_count"`
			WarningCount int `json:"warning_count"`
		} `json:"summary"`
		Findings    []gateSkillLintArtifactFinding `json:"findings"`
		RoutingRisk *lint.RoutingRiskSummary       `json:"routing_risk,omitempty"`
	} `json:"lint"`
	Eval struct {
		Suite   domaineval.RoutingEvalSuiteInfo    `json:"suite"`
		Backend domaineval.RoutingEvalBackendInfo  `json:"backend"`
		Summary domaineval.RoutingEvalSummary      `json:"summary"`
		Results []domaineval.RoutingEvalCaseResult `json:"results"`
	} `json:"eval"`
}

type gateSkillLintCompareArtifact struct {
	Run struct {
		BaseTarget      string `json:"base_target"`
		CandidateTarget string `json:"candidate_target"`
	} `json:"run"`
	Candidate struct {
		ErrorCount   int `json:"error_count"`
		WarningCount int `json:"warning_count"`
	} `json:"candidate"`
	Comparison       lint.ReportComparisonSummary  `json:"comparison"`
	AddedFindings    []gateSkillLintCompareFinding `json:"added_findings,omitempty"`
	ChangedFindings  []gateSkillLintCompareChanged `json:"changed_findings,omitempty"`
	RoutingRiskDelta *lint.RoutingRiskDelta        `json:"routing_risk_delta,omitempty"`
}

type gateSkillLintCompareFinding struct {
	RuleID   string `json:"rule_id"`
	Category string `json:"category,omitempty"`
	Severity string `json:"severity"`
}

type gateSkillLintCompareChanged struct {
	RuleID            string `json:"rule_id"`
	Category          string `json:"category,omitempty"`
	BaseSeverity      string `json:"base_severity"`
	CandidateSeverity string `json:"candidate_severity"`
}

type gateSkillEvalArtifact struct {
	Run struct {
		Target  string `json:"target"`
		Profile string `json:"profile,omitempty"`
		Runner  string `json:"runner,omitempty"`
	} `json:"run"`
	Suite   domaineval.RoutingEvalSuiteInfo    `json:"suite"`
	Backend domaineval.RoutingEvalBackendInfo  `json:"backend"`
	Summary domaineval.RoutingEvalSummary      `json:"summary"`
	Results []domaineval.RoutingEvalCaseResult `json:"results"`
}

type gateSkillEvalCompareArtifact struct {
	Run struct {
		BaseTarget      string `json:"base_target"`
		CandidateTarget string `json:"candidate_target"`
	} `json:"run"`
	Suite           domaineval.RoutingEvalSuiteInfo         `json:"suite"`
	Backend         domaineval.RoutingEvalBackendInfo       `json:"backend"`
	Base            domaineval.RoutingEvalSideSummary       `json:"base"`
	Candidate       domaineval.RoutingEvalSideSummary       `json:"candidate"`
	Comparison      domaineval.RoutingEvalComparisonSummary `json:"comparison"`
	FlippedToFail   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_fail,omitempty"`
	FlippedToPass   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_pass,omitempty"`
	ChangedCases    []domaineval.RoutingEvalCaseChange      `json:"changed_cases,omitempty"`
	ByProfileDeltas []domaineval.RoutingEvalBreakdownDelta  `json:"by_profile_deltas,omitempty"`
	ByTagDeltas     []domaineval.RoutingEvalBreakdownDelta  `json:"by_tag_deltas,omitempty"`
}

type gateSkillEvalMultiArtifact struct {
	Run struct {
		Target string `json:"target"`
	} `json:"run"`
	Suite          domaineval.RoutingEvalSuiteInfo        `json:"suite"`
	Results        []domaineval.BackendEvalReport         `json:"results"`
	Summary        domaineval.MultiBackendEvalSummary     `json:"summary"`
	DifferingCases []domaineval.MultiBackendDifferingCase `json:"differing_cases,omitempty"`
}

type gateSkillEvalMultiCompareArtifact struct {
	Run struct {
		BaseTarget      string `json:"base_target"`
		CandidateTarget string `json:"candidate_target"`
	} `json:"run"`
	Suite                 domaineval.RoutingEvalSuiteInfo              `json:"suite"`
	Backends              []domaineval.RoutingEvalBackendInfo          `json:"backends"`
	Base                  domaineval.RoutingEvalSideSummary            `json:"base"`
	Candidate             domaineval.RoutingEvalSideSummary            `json:"candidate"`
	AggregateSummary      domaineval.MultiBackendEvalComparisonSummary `json:"aggregate_summary"`
	PerBackendDeltas      []domaineval.BackendEvalComparison           `json:"per_backend_deltas"`
	DifferingCases        []domaineval.MultiBackendEvalCaseDelta       `json:"differing_cases,omitempty"`
	WidenedDisagreements  []domaineval.MultiBackendEvalCaseDelta       `json:"widened_disagreements,omitempty"`
	NarrowedDisagreements []domaineval.MultiBackendEvalCaseDelta       `json:"narrowed_disagreements,omitempty"`
}

type gateSkillBaselineCompareArtifact struct {
	Run struct {
		BaselinePath  string `json:"baseline_path"`
		CurrentTarget string `json:"current_target"`
	} `json:"run"`
	Comparison struct {
		BaselineTarget string `json:"baseline_target"`
		CurrentTarget  string `json:"current_target"`
		CurrentSummary struct {
			ErrorCount          int                   `json:"error_count"`
			WarningCount        int                   `json:"warning_count"`
			RoutingRisk         lint.RoutingRiskLevel `json:"routing_risk"`
			HasEval             bool                  `json:"has_eval"`
			HasMultiBackendEval bool                  `json:"has_multi_backend_eval"`
		} `json:"current_summary"`
		LintComparison         *lint.ReportComparison                 `json:"lint_comparison,omitempty"`
		EvalComparison         *domaineval.RoutingEvalComparison      `json:"eval_comparison,omitempty"`
		MultiBackendComparison *domaineval.MultiBackendEvalComparison `json:"multi_backend_comparison,omitempty"`
	} `json:"comparison"`
}

type gateSkillGateArtifact struct {
	Run struct {
		Target string `json:"target"`
	} `json:"run"`
	Result domaingate.Result `json:"result"`
}
