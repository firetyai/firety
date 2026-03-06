package lint

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Finding struct {
	Severity Severity
	RuleID   string
	Message  string
	Path     string
	Line     int
}

type Report struct {
	Target   string
	Findings []Finding
}

func NewReport(target string) Report {
	return Report{Target: target}
}

func (r *Report) Add(rule Rule, message, path string, line int) {
	r.Findings = append(r.Findings, Finding{
		Severity: rule.Severity,
		RuleID:   rule.ID,
		Message:  message,
		Path:     path,
		Line:     line,
	})
}

func (r *Report) ApplyStrictness(strictness Strictness) {
	for index := range r.Findings {
		rule, ok := FindRule(r.Findings[index].RuleID)
		if !ok {
			continue
		}

		r.Findings[index].Severity = rule.SeverityFor(strictness)
	}
}

func (r *Report) AddRule(rule Rule, message, path string, line int) {
	r.Add(rule, message, path, line)
}

func (r *Report) AddError(rule Rule, message, path string, line int) {
	r.Add(rule, message, path, line)
}

func (r *Report) AddWarning(rule Rule, message, path string, line int) {
	r.Add(rule, message, path, line)
}

func (r Report) ErrorCount() int {
	var count int

	for _, finding := range r.Findings {
		if finding.Severity == SeverityError {
			count++
		}
	}

	return count
}

func (r Report) WarningCount() int {
	var count int

	for _, finding := range r.Findings {
		if finding.Severity == SeverityWarning {
			count++
		}
	}

	return count
}

func (r Report) HasErrors() bool {
	return r.ErrorCount() > 0
}
