package lint

import "fmt"

type TargetingPosture string

const (
	TargetingPosturePortable            TargetingPosture = "portable"
	TargetingPostureIntentionalTargeted TargetingPosture = "intentional-targeted"
	TargetingPostureAmbiguous           TargetingPosture = "ambiguous"
	TargetingPostureAccidentalLockIn    TargetingPosture = "accidental-lock-in"
)

type ExplainContext struct {
	GuidanceProfile      string           `json:"guidance_profile,omitempty"`
	Strictness           Strictness       `json:"strictness,omitempty"`
	TargetingPosture     TargetingPosture `json:"targeting_posture,omitempty"`
	HasPortabilityIssues bool             `json:"has_portability_issues,omitempty"`
}

type Explanation struct {
	Category            Category         `json:"category,omitempty"`
	WhyItMatters        string           `json:"why_it_matters,omitempty"`
	WhatGoodLooksLike   string           `json:"what_good_looks_like,omitempty"`
	ImprovementHint     string           `json:"improvement_hint,omitempty"`
	FixHint             string           `json:"fix_hint,omitempty"`
	GuidanceProfile     string           `json:"guidance_profile,omitempty"`
	ProfileSpecificHint string           `json:"profile_specific_hint,omitempty"`
	TargetingPosture    TargetingPosture `json:"targeting_posture,omitempty"`
	AutofixAvailable    bool             `json:"autofix_available,omitempty"`
}

func NewExplainContext(findings []Finding, profile string, strictness Strictness) ExplainContext {
	normalizedProfile := normalizeGuidanceProfile(profile)
	ctx := ExplainContext{
		GuidanceProfile: normalizedProfile,
		Strictness:      strictness,
	}

	var hasMixed bool
	var hasAccidental bool
	var hasTargeted bool

	for _, finding := range findings {
		rule, ok := FindRule(finding.RuleID)
		if !ok || !rule.ProfileAware {
			continue
		}

		ctx.HasPortabilityIssues = true

		switch finding.RuleID {
		case RuleAccidentalToolLockIn.ID, RuleGenericPortabilityContradiction.ID, RuleGenericProfileToolLocking.ID:
			hasAccidental = true
		case RuleMixedEcosystemGuidance.ID, RuleUnclearToolTargeting.ID, RuleProfileTargetMismatch.ID, RuleExampleEcosystemMismatch.ID, RuleProfileIncompatibleGuidance.ID:
			hasMixed = true
		case RuleToolSpecificBranding.ID, RuleToolSpecificInstallAssumption.ID, RuleNonportableInvocationGuidance.ID, RuleMissingToolTargetBoundary.ID:
			hasTargeted = true
		}
	}

	switch {
	case !ctx.HasPortabilityIssues && normalizedProfile == "generic":
		ctx.TargetingPosture = TargetingPosturePortable
	case !ctx.HasPortabilityIssues:
		ctx.TargetingPosture = TargetingPostureIntentionalTargeted
	case hasAccidental:
		ctx.TargetingPosture = TargetingPostureAccidentalLockIn
	case hasMixed:
		ctx.TargetingPosture = TargetingPostureAmbiguous
	case normalizedProfile == "generic" && hasTargeted:
		ctx.TargetingPosture = TargetingPostureAccidentalLockIn
	case hasTargeted:
		ctx.TargetingPosture = TargetingPostureIntentionalTargeted
	default:
		ctx.TargetingPosture = TargetingPostureAmbiguous
	}

	return ctx
}

func (r Rule) Explain(ctx ExplainContext) Explanation {
	explanation := Explanation{
		Category:          r.Category,
		WhyItMatters:      r.Why,
		WhatGoodLooksLike: r.WhatGoodLooksLike,
		ImprovementHint:   r.ImprovementHint,
		FixHint:           r.FixHint,
		AutofixAvailable:  r.Fixability == FixabilityAutomatic,
	}

	if !r.ProfileAware {
		if ctx.Strictness == StrictnessPedantic && isStrictnessSensitiveRule(r) {
			explanation.ImprovementHint = explanation.ImprovementHint + " Pedantic mode expects this guidance to be explicit and complete."
		} else if ctx.Strictness == StrictnessStrict && isStrictnessSensitiveRule(r) {
			explanation.ImprovementHint = explanation.ImprovementHint + " Strict mode expects this to be production-ready rather than implied."
		}
		return explanation
	}

	explanation.GuidanceProfile = ctx.GuidanceProfile
	explanation.TargetingPosture = ctx.TargetingPosture
	explanation.ProfileSpecificHint = profileSpecificHint(r, ctx)

	return explanation
}

func isStrictnessSensitiveRule(rule Rule) bool {
	switch rule.ID {
	case RuleMissingFrontMatterDescription.ID,
		RuleEmptyFrontMatterDescription.ID,
		RuleMissingWhenToUse.ID,
		RuleMissingNegativeGuidance.ID,
		RuleMissingExamples.ID,
		RuleMissingUsageGuidance.ID,
		RuleGenericPortabilityContradiction.ID,
		RuleProfileTargetMismatch.ID:
		return true
	default:
		return false
	}
}

func PortabilitySummary(ctx ExplainContext) string {
	if !ctx.HasPortabilityIssues {
		return ""
	}

	if ctx.GuidanceProfile == "generic" {
		switch ctx.TargetingPosture {
		case TargetingPostureAmbiguous:
			return "To stay generic, remove mixed ecosystem cues and keep examples, install notes, and invocation wording tool-neutral."
		case TargetingPostureAccidentalLockIn:
			return "To stay generic, rewrite tool-branded instructions in neutral terms or explicitly narrow the skill to the ecosystem it actually targets."
		default:
			return "To stay generic, keep the main guidance neutral and move any unavoidable tool-specific notes into clear boundary sections."
		}
	}

	displayName := guidanceProfileDisplayName(ctx.GuidanceProfile)
	switch ctx.TargetingPosture {
	case TargetingPostureAmbiguous:
		summary := fmt.Sprintf("To target %s cleanly, remove conflicting ecosystem cues and add explicit boundary language about who the skill is and is not for.", displayName)
		if ctx.Strictness == StrictnessPedantic {
			return summary + " Pedantic mode expects the target posture to be unambiguous."
		}
		return summary
	case TargetingPostureAccidentalLockIn:
		return fmt.Sprintf("Your skill currently reads as unintentionally tool-locked; either align the whole skill with %s or rewrite the instructions and examples to be neutral.", displayName)
	default:
		return fmt.Sprintf("To target %s cleanly, keep examples, invocation wording, and install assumptions aligned with %s and remove contradictory ecosystem references.", displayName, displayName)
	}
}

func profileSpecificHint(rule Rule, ctx ExplainContext) string {
	if !rule.ProfileAware {
		return ""
	}

	if ctx.GuidanceProfile == "generic" {
		switch rule.ID {
		case RuleToolSpecificBranding.ID, RuleToolSpecificInstallAssumption.ID, RuleNonportableInvocationGuidance.ID:
			return "For the generic profile, keep the main instructions tool-neutral and move any unavoidable ecosystem-specific notes into explicit boundary guidance."
		case RuleGenericProfileToolLocking.ID, RuleAccidentalToolLockIn.ID, RuleGenericPortabilityContradiction.ID:
			return "For the generic profile, replace branded terms, install paths, and invocation conventions with neutral wording unless the skill is intentionally scoped."
		case RuleMixedEcosystemGuidance.ID, RuleProfileTargetMismatch.ID, RuleExampleEcosystemMismatch.ID, RuleProfileIncompatibleGuidance.ID:
			return "For the generic profile, remove cross-tool examples and keep installation, invocation, and examples consistent without ecosystem-specific wording."
		case RuleUnclearToolTargeting.ID, RuleMissingToolTargetBoundary.ID:
			return "For the generic profile, avoid implying one tool-specific audience unless the skill is clearly and intentionally narrowed."
		default:
			return ""
		}
	}

	displayName := guidanceProfileDisplayName(ctx.GuidanceProfile)
	switch rule.ID {
	case RuleToolSpecificBranding.ID, RuleToolSpecificInstallAssumption.ID, RuleNonportableInvocationGuidance.ID:
		return fmt.Sprintf("For the %s profile, say the target explicitly and remove conflicting ecosystem cues from the main instructions.", displayName)
	case RuleGenericProfileToolLocking.ID, RuleAccidentalToolLockIn.ID, RuleGenericPortabilityContradiction.ID:
		return fmt.Sprintf("For the %s profile, either align the whole skill with %s or restate it as generic without tool-specific assumptions.", displayName, displayName)
	case RuleMixedEcosystemGuidance.ID, RuleProfileTargetMismatch.ID, RuleExampleEcosystemMismatch.ID, RuleProfileIncompatibleGuidance.ID:
		return fmt.Sprintf("For the %s profile, keep examples, installation notes, and invocation wording aligned with %s and drop conflicting ecosystem references.", displayName, displayName)
	case RuleUnclearToolTargeting.ID, RuleMissingToolTargetBoundary.ID:
		return fmt.Sprintf("For the %s profile, add a short boundary section that says this skill is for %s-style workflows and when another tool or skill is a better fit.", displayName, displayName)
	default:
		return ""
	}
}

func normalizeGuidanceProfile(profile string) string {
	switch profile {
	case "codex", "claude-code", "copilot", "cursor":
		return profile
	default:
		return "generic"
	}
}

func guidanceProfileDisplayName(profile string) string {
	switch normalizeGuidanceProfile(profile) {
	case "codex":
		return "Codex"
	case "claude-code":
		return "Claude Code"
	case "copilot":
		return "Copilot"
	case "cursor":
		return "Cursor"
	default:
		return "generic"
	}
}
