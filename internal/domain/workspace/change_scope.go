package workspace

import (
	"fmt"
	"slices"
	"strings"
)

type DiffMode string
type ScopeImpact string

const (
	DiffModeWorkingTreeVsHEAD DiffMode = "working-tree-vs-head"
	DiffModeRevisionRange     DiffMode = "revision-range"

	ImpactDirect    ScopeImpact = "direct"
	ImpactIndirect  ScopeImpact = "indirect"
	ImpactAmbiguous ScopeImpact = "ambiguous"
)

type DiffContext struct {
	Mode    DiffMode `json:"mode"`
	BaseRev string   `json:"base_rev,omitempty"`
	HeadRev string   `json:"head_rev,omitempty"`
	Summary string   `json:"summary"`
}

type SkillScopeReason struct {
	Skill   SkillRef    `json:"skill"`
	Impact  ScopeImpact `json:"impact"`
	Summary string      `json:"summary"`
	Files   []string    `json:"files,omitempty"`
}

type ChangeScope struct {
	WorkspaceRoot         string             `json:"workspace_root"`
	DiffContext           DiffContext        `json:"diff_context"`
	ChangedFiles          []string           `json:"changed_files"`
	DirectlyChangedSkills []SkillRef         `json:"directly_changed_skills,omitempty"`
	ImpactedSkills        []SkillRef         `json:"impacted_skills,omitempty"`
	UnchangedSkills       []SkillRef         `json:"unchanged_skills,omitempty"`
	SelectedSkills        []SkillRef         `json:"selected_analysis_scope,omitempty"`
	AmbiguousImpacts      []string           `json:"ambiguous_impacts,omitempty"`
	Caveats               []string           `json:"caveats,omitempty"`
	SkillReasons          []SkillScopeReason `json:"scope_reasons,omitempty"`
	Summary               string             `json:"summary"`
}

func BuildChangeScope(
	workspaceRoot string,
	diffContext DiffContext,
	changedFiles []string,
	direct []SkillRef,
	impacted []SkillRef,
	unchanged []SkillRef,
	selected []SkillRef,
	ambiguous []string,
	caveats []string,
	reasons []SkillScopeReason,
) ChangeScope {
	scope := ChangeScope{
		WorkspaceRoot:         workspaceRoot,
		DiffContext:           diffContext,
		ChangedFiles:          uniqueSortedStrings(changedFiles),
		DirectlyChangedSkills: uniqueSortedSkillRefs(direct),
		ImpactedSkills:        uniqueSortedSkillRefs(impacted),
		UnchangedSkills:       uniqueSortedSkillRefs(unchanged),
		SelectedSkills:        uniqueSortedSkillRefs(selected),
		AmbiguousImpacts:      uniqueSortedStrings(ambiguous),
		Caveats:               uniqueSortedStrings(caveats),
		SkillReasons:          uniqueSortedSkillReasons(reasons),
	}
	scope.Summary = summarizeChangeScope(scope)
	return scope
}

func summarizeChangeScope(scope ChangeScope) string {
	parts := []string{
		fmt.Sprintf("%d changed file(s)", len(scope.ChangedFiles)),
		fmt.Sprintf("%d directly changed skill(s)", len(scope.DirectlyChangedSkills)),
	}
	if len(scope.ImpactedSkills) > 0 {
		parts = append(parts, fmt.Sprintf("%d indirectly impacted skill(s)", len(scope.ImpactedSkills)))
	}
	if len(scope.UnchangedSkills) > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged skill(s) skipped", len(scope.UnchangedSkills)))
	}
	if len(scope.AmbiguousImpacts) > 0 {
		parts = append(parts, fmt.Sprintf("%d ambiguous impact area(s)", len(scope.AmbiguousImpacts)))
	}
	return strings.Join(parts, "; ") + "."
}

func uniqueSortedSkillRefs(values []SkillRef) []SkillRef {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]SkillRef, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.Path) == "" {
			continue
		}
		seen[value.Path] = value
	}
	out := make([]SkillRef, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	slices.SortFunc(out, func(a, b SkillRef) int {
		return strings.Compare(a.Path, b.Path)
	})
	return out
}

func uniqueSortedSkillReasons(values []SkillScopeReason) []SkillScopeReason {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]SkillScopeReason, len(values))
	for _, value := range values {
		key := string(value.Impact) + "|" + value.Skill.Path + "|" + strings.TrimSpace(value.Summary)
		if strings.TrimSpace(value.Skill.Path) == "" || strings.TrimSpace(value.Summary) == "" {
			continue
		}
		value.Files = uniqueSortedStrings(value.Files)
		seen[key] = value
	}
	out := make([]SkillScopeReason, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	slices.SortFunc(out, func(a, b SkillScopeReason) int {
		if compare := strings.Compare(a.Skill.Path, b.Skill.Path); compare != 0 {
			return compare
		}
		if compare := strings.Compare(string(a.Impact), string(b.Impact)); compare != 0 {
			return compare
		}
		return strings.Compare(a.Summary, b.Summary)
	})
	return out
}
