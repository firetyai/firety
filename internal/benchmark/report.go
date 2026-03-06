package benchmark

type SuiteInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	FixtureCount int    `json:"fixture_count"`
}

type FixtureResult struct {
	Name                   string          `json:"name"`
	Intent                 string          `json:"intent"`
	Category               FixtureCategory `json:"category"`
	CategoryLabel          string          `json:"category_label"`
	Profile                string          `json:"profile,omitempty"`
	Strictness             string          `json:"strictness,omitempty"`
	Passed                 bool            `json:"passed"`
	Deterministic          bool            `json:"deterministic"`
	ErrorCount             int             `json:"error_count"`
	WarningCount           int             `json:"warning_count"`
	RoutingRisk            string          `json:"routing_risk,omitempty"`
	RoutingRiskAreas       []string        `json:"routing_risk_areas,omitempty"`
	MissingRequiredRuleIDs []string        `json:"missing_required_rule_ids,omitempty"`
	UnexpectedRuleIDs      []string        `json:"unexpected_rule_ids,omitempty"`
	RegressionIssues       []string        `json:"regression_issues,omitempty"`
	NoiseIssues            []string        `json:"noise_issues,omitempty"`
	Summary                string          `json:"summary"`
}

type CategorySummary struct {
	Category      FixtureCategory `json:"category"`
	CategoryLabel string          `json:"category_label"`
	FixtureCount  int             `json:"fixture_count"`
	Passed        int             `json:"passed"`
	Failed        int             `json:"failed"`
}

type Summary struct {
	TotalFixtures      int      `json:"total_fixtures"`
	PassedFixtures     int      `json:"passed_fixtures"`
	FailedFixtures     int      `json:"failed_fixtures"`
	SkippedFixtures    int      `json:"skipped_fixtures"`
	DeterministicCount int      `json:"deterministic_count"`
	StabilityOK        bool     `json:"stability_ok"`
	NotableRegressions []string `json:"notable_regressions,omitempty"`
	NotableNoise       []string `json:"notable_noise,omitempty"`
	ConfidenceSignals  []string `json:"confidence_signals,omitempty"`
	Summary            string   `json:"summary"`
}

type Report struct {
	Suite      SuiteInfo         `json:"suite"`
	Fixtures   []FixtureResult   `json:"fixtures"`
	Categories []CategorySummary `json:"categories"`
	Summary    Summary           `json:"summary"`
}
