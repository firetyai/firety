package lint

import "strings"

type Category string
type Fixability string

const (
	CategoryStructure        Category = "structure"
	CategoryMetadataSpec     Category = "metadata-spec"
	CategoryInvocation       Category = "invocation"
	CategoryExamples         Category = "examples"
	CategoryNegativeGuidance Category = "negative-guidance"
	CategoryConsistency      Category = "consistency"
	CategoryPortability      Category = "portability"
	CategoryBundleResources  Category = "bundle-resources"
	CategoryEfficiencyCost   Category = "efficiency-cost"
	CategoryTriggerQuality   Category = "trigger-quality"

	FixabilityNone      Fixability = "none"
	FixabilityAutomatic Fixability = "automatic"
)

type Rule struct {
	ID                string     `json:"id"`
	Slug              string     `json:"slug"`
	Category          Category   `json:"category"`
	Severity          Severity   `json:"default_severity"`
	StrictSeverity    Severity   `json:"strict_severity,omitempty"`
	PedanticSeverity  Severity   `json:"pedantic_severity,omitempty"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Why               string     `json:"why_it_matters"`
	WhatGoodLooksLike string     `json:"what_good_looks_like"`
	ImprovementHint   string     `json:"improvement_hint"`
	FixHint           string     `json:"fix_hint,omitempty"`
	Notes             []string   `json:"notes,omitempty"`
	ProfileAware      bool       `json:"profile_aware"`
	LineAware         bool       `json:"line_aware"`
	Fixability        Fixability `json:"fixability"`
}

func newRule(id string, category Category, severity Severity, title, description, why string, profileAware, lineAware bool, fixability Fixability, notes ...string) Rule {
	whatGoodLooksLike, improvementHint := defaultRuleGuidance(category)

	rule := Rule{
		ID:                id,
		Slug:              ruleSlug(id),
		Category:          category,
		Severity:          severity,
		Title:             title,
		Description:       description,
		Why:               why,
		WhatGoodLooksLike: whatGoodLooksLike,
		ImprovementHint:   improvementHint,
		Notes:             notes,
		ProfileAware:      profileAware,
		LineAware:         lineAware,
		Fixability:        fixability,
	}
	if fixability == FixabilityAutomatic {
		rule.FixHint = "Run `firety skill lint --fix` to apply Firety's supported safe mechanical fix for this rule."
	}

	return rule
}

func ruleSlug(id string) string {
	replacer := strings.NewReplacer(".", "-", "_", "-", "/", "-")
	return replacer.Replace(id)
}

func defaultRuleGuidance(category Category) (string, string) {
	switch category {
	case CategoryStructure:
		return "The skill bundle should have a readable entry document, a clear title, and working local references.", "Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout."
	case CategoryMetadataSpec:
		return "Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.", "Tighten the front matter so the name and description make the skill easier to identify, route, and catalog."
	case CategoryInvocation:
		return "A strong skill explains when it should be used and what request or input pattern should trigger it.", "Add direct usage guidance that names the trigger situations and shows how the skill is meant to be invoked."
	case CategoryExamples:
		return "Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.", "Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill."
	case CategoryNegativeGuidance:
		return "A strong skill explains where its boundaries are and when another approach should be used instead.", "Add clear limits, out-of-scope cases, or handoff guidance so misuse is less likely."
	case CategoryConsistency:
		return "The metadata, title, and body should all describe the same skill identity, scope, and intended use.", "Align the metadata and body so the skill presents one coherent purpose and boundary."
	case CategoryPortability:
		return "A portable skill either stays generic across tools or states its intended tool target and limits explicitly.", "Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly."
	case CategoryBundleResources:
		return "A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.", "Align the documented local resources with the actual bundle contents and remove brittle or misleading references."
	case CategoryEfficiencyCost:
		return "A well-sized skill keeps the core instructions and examples focused enough to stay cheap to load and easy to maintain.", "Trim repeated or oversized content so the skill stays more focused, cheaper to load, and easier to evolve."
	case CategoryTriggerQuality:
		return "A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.", "Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time."
	default:
		return "The skill should communicate its purpose, boundaries, and supporting materials clearly.", "Revise the skill so the reported issue no longer blocks clarity, reliability, or maintainability."
	}
}

func (r Rule) WithGuidance(whatGoodLooksLike, improvementHint string) Rule {
	r.WhatGoodLooksLike = whatGoodLooksLike
	r.ImprovementHint = improvementHint
	return r
}

func (r Rule) WithFixHint(fixHint string) Rule {
	r.FixHint = fixHint
	return r
}

func (r Rule) WithStrictnessSeverity(strictSeverity, pedanticSeverity Severity) Rule {
	r.StrictSeverity = strictSeverity
	r.PedanticSeverity = pedanticSeverity
	return r
}

func (r Rule) SeverityFor(strictness Strictness) Severity {
	switch strictness {
	case StrictnessPedantic:
		if r.PedanticSeverity != "" {
			return r.PedanticSeverity
		}
		if r.StrictSeverity != "" {
			return r.StrictSeverity
		}
	case StrictnessStrict:
		if r.StrictSeverity != "" {
			return r.StrictSeverity
		}
	}

	return r.Severity
}

func FindRule(id string) (Rule, bool) {
	for _, rule := range AllRules() {
		if rule.ID == id {
			return rule, true
		}
	}

	return Rule{}, false
}

var (
	RuleTargetNotFound                   = newRule("skill.target-not-found", CategoryStructure, SeverityError, "Missing target path", "Target path does not exist.", "Firety cannot lint a directory that is not present.", false, false, FixabilityNone)
	RuleTargetNotDirectory               = newRule("skill.target-not-directory", CategoryStructure, SeverityError, "Target is not a directory", "Target path is not a directory.", "Firety only lints skill directories, not standalone files.", false, false, FixabilityNone)
	RuleMissingSkillMD                   = newRule("skill.missing-skill-md", CategoryStructure, SeverityError, "Missing SKILL.md", "SKILL.md is missing.", "A skill bundle needs a SKILL.md entrypoint for Firety to lint it.", false, false, FixabilityNone)
	RuleUnreadableSkillMD                = newRule("skill.unreadable-skill-md", CategoryStructure, SeverityError, "Unreadable SKILL.md", "SKILL.md cannot be read.", "Firety cannot inspect a skill definition if the entry document is unreadable.", false, false, FixabilityNone)
	RuleEmptySkillMD                     = newRule("skill.empty-skill-md", CategoryStructure, SeverityError, "Empty SKILL.md", "SKILL.md is empty.", "An empty skill definition cannot communicate scope, usage, or examples.", false, false, FixabilityNone)
	RuleInvalidFrontMatter               = newRule("skill.invalid-front-matter", CategoryMetadataSpec, SeverityError, "Invalid front matter", "SKILL.md front matter is malformed.", "Broken front matter makes metadata unreliable for humans and automation.", false, true, FixabilityNone)
	RuleMissingFrontMatterName           = newRule("skill.missing-front-matter-name", CategoryMetadataSpec, SeverityError, "Missing front matter name", "Front matter is missing a name field.", "A missing name weakens discoverability and downstream cataloging.", false, true, FixabilityNone)
	RuleEmptyFrontMatterName             = newRule("skill.empty-front-matter-name", CategoryMetadataSpec, SeverityError, "Empty front matter name", "Front matter name is empty.", "An empty name prevents the skill from being identified clearly.", false, true, FixabilityNone)
	RuleMissingFrontMatterDescription    = newRule("skill.missing-front-matter-description", CategoryMetadataSpec, SeverityWarning, "Missing front matter description", "Front matter is missing a description field.", "A missing description makes the skill harder to route and catalog.", false, true, FixabilityNone).WithStrictnessSeverity(SeverityError, SeverityError)
	RuleEmptyFrontMatterDescription      = newRule("skill.empty-front-matter-description", CategoryMetadataSpec, SeverityWarning, "Empty front matter description", "Front matter description is empty.", "An empty description weakens routing, search, and documentation quality.", false, true, FixabilityNone).WithStrictnessSeverity(SeverityError, SeverityError)
	RuleLongFrontMatterName              = newRule("skill.long-front-matter-name", CategoryMetadataSpec, SeverityWarning, "Long front matter name", "Front matter name is unusually long.", "Very long names are harder to scan in catalogs and selection UIs.", false, true, FixabilityNone)
	RuleShortFrontMatterDescription      = newRule("skill.short-front-matter-description", CategoryMetadataSpec, SeverityWarning, "Short front matter description", "Front matter description is too short to be useful.", "Very short descriptions rarely explain a skill's purpose or boundaries well.", false, true, FixabilityNone)
	RuleLongFrontMatterDescription       = newRule("skill.long-front-matter-description", CategoryMetadataSpec, SeverityWarning, "Long front matter description", "Front matter description is excessively long.", "Overlong descriptions often hide the core trigger and make metadata noisy.", false, true, FixabilityNone)
	RuleVagueDescription                 = newRule("skill.vague-description", CategoryMetadataSpec, SeverityWarning, "Vague description", "Description is too vague to clearly distinguish the skill.", "Generic descriptions make it harder for users and agents to know when this skill is the right fit.", false, true, FixabilityNone).WithGuidance("The description should quickly say what the skill does, when it should be used, and what makes it distinct.", "Replace generic phrasing with one concrete purpose statement that names the trigger and the kind of outcome the skill is meant to produce.")
	RuleNameTitleMismatch                = newRule("skill.name-title-mismatch", CategoryConsistency, SeverityWarning, "Name and title mismatch", "Front matter name appears inconsistent with the title.", "Conflicting identity signals make the skill feel sloppy and harder to trust.", false, true, FixabilityNone)
	RuleDescriptionBodyMismatch          = newRule("skill.description-body-mismatch", CategoryConsistency, SeverityWarning, "Description and body mismatch", "Front matter description appears inconsistent with the body.", "Conflicting metadata and body content undermine discoverability and routing.", false, true, FixabilityNone)
	RuleScopeMismatch                    = newRule("skill.scope-mismatch", CategoryConsistency, SeverityWarning, "Scope mismatch", "Front matter scope appears inconsistent with the body.", "A skill should present one clear scope across metadata and instructions.", false, true, FixabilityNone)
	RuleGenericName                      = newRule("skill.generic-name", CategoryTriggerQuality, SeverityWarning, "Generic skill name", "Skill name is too generic to be distinctive.", "A generic name weakens discoverability and trigger quality.", false, true, FixabilityNone).WithGuidance("A strong skill name is short, specific, and distinctive enough that a user or agent can tell what it is for at a glance.", "Rename the skill toward the concrete task or domain it handles instead of using broad labels like helper, assistant, or tool.")
	RuleGenericTriggerDescription        = newRule("skill.generic-trigger-description", CategoryTriggerQuality, SeverityWarning, "Generic trigger description", "Trigger description is too generic to distinguish the skill.", "If the description sounds like a general assistant, routing becomes less reliable.", false, true, FixabilityNone)
	RuleDiffuseScope                     = newRule("skill.diffuse-scope", CategoryTriggerQuality, SeverityWarning, "Diffuse scope", "Skill scope appears too broad or diffuse.", "Skills with scattered scope are harder to trigger at the right time.", false, true, FixabilityNone)
	RuleOverbroadWhenToUse               = newRule("skill.overbroad-when-to-use", CategoryTriggerQuality, SeverityWarning, "Overbroad when-to-use guidance", "When-to-use guidance appears too broad to route clearly.", "Overbroad guidance makes the skill compete with generic assistant behavior.", false, true, FixabilityNone)
	RuleWeakTriggerPattern               = newRule("skill.weak-trigger-pattern", CategoryTriggerQuality, SeverityWarning, "Weak trigger pattern", "Examples do not reinforce a clear trigger pattern.", "Examples should help reinforce when this skill should be selected.", false, true, FixabilityNone)
	RuleLowDistinctiveness               = newRule("skill.low-distinctiveness", CategoryTriggerQuality, SeverityWarning, "Low distinctiveness", "The skill lacks distinctive trigger terms or phrases.", "Distinctive language helps the skill stand out from neighboring capabilities.", false, true, FixabilityNone)
	RuleExampleTriggerMismatch           = newRule("skill.example-trigger-mismatch", CategoryTriggerQuality, SeverityWarning, "Example trigger mismatch", "Examples appear misaligned with the documented trigger concept.", "Misaligned examples weaken trust in the skill's stated purpose.", false, true, FixabilityNone)
	RuleTriggerScopeInconsistency        = newRule("skill.trigger-scope-inconsistency", CategoryTriggerQuality, SeverityWarning, "Trigger scope inconsistency", "Name, description, and body point to different trigger concepts.", "Trigger quality depends on one coherent concept across the whole document.", false, true, FixabilityNone)
	RuleMissingTitle                     = newRule("skill.missing-title", CategoryStructure, SeverityError, "Missing top-level title", "SKILL.md is missing a top-level markdown title.", "A missing title weakens readability and makes the skill harder to identify.", false, true, FixabilityAutomatic).WithGuidance("The document should start with one clear `#` title that matches the skill identity used everywhere else.", "Add a single top-level title near the start of `SKILL.md`, then keep the rest of the headings under it.").WithFixHint("Run `firety skill lint --fix` to insert a safe placeholder title, then edit it if needed.")
	RuleBrokenLocalLink                  = newRule("skill.broken-local-link", CategoryStructure, SeverityError, "Broken local link", "Local markdown link points to a missing file.", "Broken links make a skill feel incomplete and can hide missing supporting material.", false, true, FixabilityNone)
	RuleReferenceOutsideRoot             = newRule("skill.reference-outside-root", CategoryBundleResources, SeverityWarning, "Reference outside skill root", "Referenced resource escapes the skill root.", "References outside the bundle are brittle and reduce portability.", false, true, FixabilityNone)
	RuleReferencedDirectoryInsteadOfFile = newRule("skill.referenced-directory-instead-of-file", CategoryBundleResources, SeverityWarning, "Directory referenced as a file", "Referenced resource is a directory where a file is expected.", "A directory link often means the bundle structure or documentation is unclear.", false, true, FixabilityNone)
	RuleEmptyReferencedResource          = newRule("skill.empty-referenced-resource", CategoryBundleResources, SeverityWarning, "Empty referenced resource", "Referenced resource exists but is empty.", "An empty supporting file is usually accidental and not helpful to consumers.", false, true, FixabilityNone)
	RuleSuspiciousReferencedResource     = newRule("skill.suspicious-referenced-resource", CategoryBundleResources, SeverityWarning, "Suspicious referenced resource", "Referenced resource type looks suspicious for a skill bundle.", "Unexpected binary-style resources can make a bundle harder to trust or reuse.", false, true, FixabilityNone)
	RuleDuplicateResourceReference       = newRule("skill.duplicate-resource-reference", CategoryBundleResources, SeverityWarning, "Duplicate resource reference", "The same local resource is referenced repeatedly.", "Repeated references often signal clutter or accidental duplication in the document.", false, true, FixabilityNone)
	RuleMissingMentionedResource         = newRule("skill.missing-mentioned-resource", CategoryBundleResources, SeverityWarning, "Missing mentioned resource", "A strongly-mentioned local resource is missing from the bundle.", "If SKILL.md names a local helper, it should generally exist in the package.", false, true, FixabilityNone)
	RuleInconsistentBundleStructure      = newRule("skill.inconsistent-bundle-structure", CategoryBundleResources, SeverityWarning, "Inconsistent bundle structure", "The skill bundle structure appears inconsistent with the documentation.", "A bundle should match what the skill says is available.", false, false, FixabilityNone)
	RulePossiblyStaleResource            = newRule("skill.possibly-stale-resource", CategoryBundleResources, SeverityWarning, "Possibly stale resource", "The bundle contains helper resources that may be stale or unused.", "Stale bundle contents add maintenance cost and reduce trust in the package.", false, false, FixabilityNone)
	RuleUnhelpfulReferencedResource      = newRule("skill.unhelpful-referenced-resource", CategoryBundleResources, SeverityWarning, "Unhelpful referenced resource", "Referenced resource exists but appears too short to be useful.", "A referenced helper should provide enough substance to justify including it.", false, true, FixabilityNone)
	RuleLargeSkillMD                     = newRule("skill.large-skill-md", CategoryEfficiencyCost, SeverityWarning, "Large SKILL.md", "SKILL.md appears large enough to create unnecessary context cost.", "Large instructions increase maintenance cost and likely token usage.", false, false, FixabilityNone).WithGuidance("The main skill document should stay focused enough that an agent can load it without carrying a large amount of avoidable context.", "Trim repeated prose, move optional detail into focused supporting resources, and keep `SKILL.md` centered on the core trigger, guidance, and examples.")
	RuleExcessiveExampleVolume           = newRule("skill.excessive-example-volume", CategoryEfficiencyCost, SeverityWarning, "Excessive example volume", "Examples appear unusually large for a single skill.", "Too many example tokens can crowd out the core instructions.", false, true, FixabilityNone)
	RuleDuplicateExamples                = newRule("skill.duplicate-examples", CategoryEfficiencyCost, SeverityWarning, "Duplicate examples", "Examples appear duplicated or near-duplicated.", "Repeated examples add cost without improving understanding.", false, true, FixabilityNone)
	RuleLargeReferencedResource          = newRule("skill.large-referenced-resource", CategoryEfficiencyCost, SeverityWarning, "Large referenced resource", "A referenced resource appears large enough to be costly to load.", "Very large referenced text files can make a skill expensive to use.", false, true, FixabilityNone)
	RuleExcessiveBundleSize              = newRule("skill.excessive-bundle-size", CategoryEfficiencyCost, SeverityWarning, "Excessive bundle size", "The likely-loaded skill bundle appears excessively large.", "A single skill should usually stay compact enough to load cheaply.", false, false, FixabilityNone)
	RuleRepetitiveInstructions           = newRule("skill.repetitive-instructions", CategoryEfficiencyCost, SeverityWarning, "Repetitive instructions", "Instruction sections appear overly repetitive.", "Repeated instruction text increases cost without improving clarity.", false, true, FixabilityNone)
	RuleUnbalancedSkillContent           = newRule("skill.unbalanced-skill-content", CategoryEfficiencyCost, SeverityWarning, "Unbalanced skill content", "The skill appears unbalanced between instructions and examples.", "Extreme imbalance can make a skill either vague or unnecessarily expensive.", false, true, FixabilityNone)
	RuleLargeContent                     = newRule("skill.large-content", CategoryStructure, SeverityWarning, "Large markdown content", "SKILL.md is very large and may be hard to maintain.", "Very large documents are harder to review and evolve cleanly.", false, false, FixabilityNone)
	RuleDuplicateHeading                 = newRule("skill.duplicate-heading", CategoryStructure, SeverityWarning, "Duplicate heading", "Duplicate heading found.", "Duplicate headings make the document harder to navigate and maintain.", false, true, FixabilityNone)
	RuleMissingWhenToUse                 = newRule("skill.missing-when-to-use", CategoryInvocation, SeverityWarning, "Missing when-to-use guidance", "No obvious guidance explains when to use the skill.", "A skill should explain its intended trigger situations directly.", false, false, FixabilityNone).WithGuidance("A strong skill says when it should be selected, not just what it contains.", "Add a short `When to use` section that names the user requests, task shapes, or situations that should trigger the skill.").WithStrictnessSeverity(SeverityError, SeverityError)
	RuleMissingNegativeGuidance          = newRule("skill.missing-negative-guidance", CategoryNegativeGuidance, SeverityWarning, "Missing negative guidance", "No obvious guidance explains when not to use the skill.", "Boundary guidance prevents misuse and improves routing quality.", false, false, FixabilityNone).WithGuidance("A trustworthy skill explains both its fit and its boundaries so it does not compete with generic assistant behavior.", "Add a short limitations or `When not to use` section that names out-of-scope work, preferred alternatives, or handoff cases.").WithStrictnessSeverity("", SeverityError)
	RuleWeakNegativeGuidance             = newRule("skill.weak-negative-guidance", CategoryNegativeGuidance, SeverityWarning, "Weak negative guidance", "Negative guidance exists but appears too weak or generic.", "Weak boundaries leave too much ambiguity about when the skill does not fit.", false, true, FixabilityNone)
	RuleMissingExamples                  = newRule("skill.missing-examples", CategoryExamples, SeverityWarning, "Missing examples", "No obvious examples section found.", "Examples make a skill easier to understand and use correctly.", false, false, FixabilityNone).WithGuidance("At least one realistic example should show what kind of request triggers the skill and what a good result looks like.", "Add a small examples section with one or two concrete requests, the expected invocation pattern, and the outcome or output style.").WithStrictnessSeverity("", SeverityError)
	RuleWeakExamples                     = newRule("skill.weak-examples", CategoryExamples, SeverityWarning, "Weak examples", "Examples exist but appear too short or content-light.", "Thin examples often fail to show a realistic usage pattern.", false, true, FixabilityNone)
	RuleGenericExamples                  = newRule("skill.generic-examples", CategoryExamples, SeverityWarning, "Generic examples", "Examples exist but appear too generic.", "Examples should make the skill feel concrete, not interchangeable.", false, true, FixabilityNone)
	RuleExamplesMissingInvocationPattern = newRule("skill.examples-missing-invocation-pattern", CategoryExamples, SeverityWarning, "Examples missing invocation pattern", "Examples do not show a clear invocation or request pattern.", "Examples should show how the skill gets triggered in practice.", false, true, FixabilityNone)
	RuleAbstractExamples                 = newRule("skill.abstract-examples", CategoryExamples, SeverityWarning, "Abstract examples", "Examples are too abstract to be practically useful.", "Abstract examples rarely help a user or agent apply the skill correctly.", false, true, FixabilityNone)
	RulePlaceholderHeavyExamples         = newRule("skill.placeholder-heavy-examples", CategoryExamples, SeverityWarning, "Placeholder-heavy examples", "Examples rely too heavily on placeholders instead of concrete values.", "Too many placeholders make examples feel unfinished and hard to follow.", false, true, FixabilityNone)
	RuleExamplesMissingExpectedOutcome   = newRule("skill.examples-missing-expected-outcome", CategoryExamples, SeverityWarning, "Examples missing expected outcome", "Examples show a trigger but not the expected outcome or output style.", "Good examples usually show both how to start and what success looks like.", false, true, FixabilityNone)
	RuleExamplesMissingTriggerInput      = newRule("skill.examples-missing-trigger-input", CategoryExamples, SeverityWarning, "Examples missing trigger input", "Examples show outcomes without a clear triggering input.", "Output-only examples make it harder to understand how to invoke the skill.", false, true, FixabilityNone)
	RuleExampleScopeContradiction        = newRule("skill.example-scope-contradiction", CategoryExamples, SeverityWarning, "Example scope contradiction", "Examples appear to contradict the skill's documented scope or limitations.", "Examples should reinforce the documented scope rather than undermine it.", false, true, FixabilityNone)
	RuleExampleGuidanceMismatch          = newRule("skill.example-guidance-mismatch", CategoryExamples, SeverityWarning, "Example guidance mismatch", "Examples appear inconsistent with the when-to-use guidance.", "If examples and guidance disagree, the skill's usage becomes ambiguous.", false, true, FixabilityNone)
	RuleIncompleteExample                = newRule("skill.incomplete-example", CategoryExamples, SeverityWarning, "Incomplete example", "An example appears incomplete or abruptly truncated.", "Incomplete examples make the skill feel unfinished and unreliable.", false, true, FixabilityNone)
	RuleExampleMissingBundleResource     = newRule("skill.example-missing-bundle-resource", CategoryExamples, SeverityWarning, "Example missing bundle resource", "An example references a local bundle resource that is missing.", "Examples should not rely on local files that are absent from the bundle.", false, true, FixabilityNone)
	RuleLowVarietyExamples               = newRule("skill.low-variety-examples", CategoryExamples, SeverityWarning, "Low-variety examples", "Examples do not demonstrate enough realistic variation.", "A small amount of realistic variety helps show the skill's intended use space.", false, true, FixabilityNone)
	RuleMissingUsageGuidance             = newRule("skill.missing-usage-guidance", CategoryInvocation, SeverityWarning, "Missing usage guidance", "No obvious usage or invocation guidance found.", "A skill should explain how to invoke it or what inputs it expects.", false, false, FixabilityNone).WithGuidance("A usable skill explains what information or request pattern the caller should provide.", "Add a short usage section that states the expected inputs, invocation framing, or key parameters the skill needs to work well.").WithStrictnessSeverity(SeverityError, SeverityError)
	RuleToolSpecificBranding             = newRule("skill.tool-specific-branding", CategoryPortability, SeverityWarning, "Tool-specific branding", "Instructions are strongly branded around a specific tool ecosystem.", "Heavy branding can make a skill feel less portable unless the targeting is intentional and clear.", true, true, FixabilityNone).WithGuidance("If the skill is truly portable, the wording should stay mostly tool-neutral. If it is targeted, the target should be explicit and consistent.", "Either remove unnecessary tool branding or add clear audience and boundary language so the targeting looks intentional rather than accidental.")
	RuleProfileIncompatibleGuidance      = newRule("skill.profile-incompatible-guidance", CategoryPortability, SeverityWarning, "Profile-incompatible guidance", "Guidance appears incompatible with the selected portability profile.", "Conflicting profile guidance makes the skill harder to reuse safely across ecosystems.", true, true, FixabilityNone)
	RuleToolSpecificInstallAssumption    = newRule("skill.tool-specific-install-assumption", CategoryPortability, SeverityWarning, "Tool-specific install assumption", "Instructions assume a tool-specific install location or filesystem layout.", "Hard-coded ecosystem paths reduce portability and often surprise users.", true, true, FixabilityNone)
	RuleNonportableInvocationGuidance    = newRule("skill.nonportable-invocation-guidance", CategoryPortability, SeverityWarning, "Nonportable invocation guidance", "Invocation guidance depends on tool-specific commands or UX conventions.", "Tool-specific invocation language can accidentally lock a skill to one ecosystem.", true, true, FixabilityNone)
	RuleGenericProfileToolLocking        = newRule("skill.generic-profile-tool-locking", CategoryPortability, SeverityWarning, "Generic profile tool locking", "The skill appears too tightly coupled to one tool for the generic profile.", "The generic profile should stay reserved for skills that read as broadly portable.", true, true, FixabilityNone)
	RuleUnclearToolTargeting             = newRule("skill.unclear-tool-targeting", CategoryPortability, SeverityWarning, "Unclear tool targeting", "The skill uses tool-specific conventions without clearly stating its intended target.", "If a skill is targeted, it should say so explicitly instead of surprising the reader later.", true, true, FixabilityNone)
	RuleAccidentalToolLockIn             = newRule("skill.accidental-tool-lock-in", CategoryPortability, SeverityWarning, "Accidental tool lock-in", "The skill appears unintentionally locked to one tool ecosystem.", "Accidental lock-in is usually worse than honest targeting because it is harder to reason about.", true, true, FixabilityNone).WithGuidance("A targeted skill should say so clearly; a portable skill should avoid depending on one ecosystem's commands, paths, or UX metaphors.", "Decide whether the skill is truly generic or intentionally targeted, then make the wording, examples, and boundary guidance match that choice.")
	RuleGenericPortabilityContradiction  = newRule("skill.generic-portability-contradiction", CategoryPortability, SeverityWarning, "Generic portability contradiction", "The skill claims to be generic or portable but behaves as tool-specific.", "Contradictory portability claims make the skill hard to trust in automation and CI.", true, true, FixabilityNone).WithGuidance("A skill's stated portability posture should match its actual instructions, examples, and install assumptions.", "Either make the wording and examples more tool-neutral or explicitly narrow the skill to the ecosystem it really targets.").WithStrictnessSeverity(SeverityError, SeverityError)
	RuleMixedEcosystemGuidance           = newRule("skill.mixed-ecosystem-guidance", CategoryPortability, SeverityWarning, "Mixed ecosystem guidance", "Guidance mixes multiple tool ecosystems in a confusing way.", "Mixed instructions make it unclear which runtime or workflow the skill actually targets.", true, true, FixabilityNone)
	RuleMissingToolTargetBoundary        = newRule("skill.missing-tool-target-boundary", CategoryPortability, SeverityWarning, "Missing tool-target boundary", "A tool-specific skill lacks explicit boundary guidance about its intended audience.", "Honest tool targeting should also explain who the skill is and is not for.", true, true, FixabilityNone)
	RuleProfileTargetMismatch            = newRule("skill.profile-target-mismatch", CategoryPortability, SeverityWarning, "Profile target mismatch", "The selected profile conflicts with the skill's apparent intended target.", "A strong profile mismatch suggests the skill is being evaluated under the wrong portability posture.", true, true, FixabilityNone).WithStrictnessSeverity(SeverityError, SeverityError)
	RuleExampleEcosystemMismatch         = newRule("skill.example-ecosystem-mismatch", CategoryPortability, SeverityWarning, "Example ecosystem mismatch", "Examples reinforce a different tool ecosystem than the rest of the skill.", "Examples should support the same portability posture as the rest of the document.", true, true, FixabilityNone)
	RuleSuspiciousRelativePath           = newRule("skill.suspicious-relative-path", CategoryStructure, SeverityWarning, "Suspicious relative path", "Relative path looks suspicious.", "Odd relative paths often signal portability or maintenance problems.", false, true, FixabilityNone)
	RuleShortContent                     = newRule("skill.short-content", CategoryStructure, SeverityWarning, "Very short content", "SKILL.md content is very short and may not be useful.", "Very short skills rarely contain enough guidance to be trustworthy or reusable.", false, false, FixabilityNone)
)

func AllRules() []Rule {
	return []Rule{
		RuleTargetNotFound,
		RuleTargetNotDirectory,
		RuleMissingSkillMD,
		RuleUnreadableSkillMD,
		RuleEmptySkillMD,
		RuleInvalidFrontMatter,
		RuleMissingFrontMatterName,
		RuleEmptyFrontMatterName,
		RuleMissingFrontMatterDescription,
		RuleEmptyFrontMatterDescription,
		RuleLongFrontMatterName,
		RuleShortFrontMatterDescription,
		RuleLongFrontMatterDescription,
		RuleVagueDescription,
		RuleNameTitleMismatch,
		RuleDescriptionBodyMismatch,
		RuleScopeMismatch,
		RuleGenericName,
		RuleGenericTriggerDescription,
		RuleDiffuseScope,
		RuleOverbroadWhenToUse,
		RuleWeakTriggerPattern,
		RuleLowDistinctiveness,
		RuleExampleTriggerMismatch,
		RuleTriggerScopeInconsistency,
		RuleMissingTitle,
		RuleBrokenLocalLink,
		RuleReferenceOutsideRoot,
		RuleReferencedDirectoryInsteadOfFile,
		RuleEmptyReferencedResource,
		RuleSuspiciousReferencedResource,
		RuleDuplicateResourceReference,
		RuleMissingMentionedResource,
		RuleInconsistentBundleStructure,
		RulePossiblyStaleResource,
		RuleUnhelpfulReferencedResource,
		RuleLargeSkillMD,
		RuleExcessiveExampleVolume,
		RuleDuplicateExamples,
		RuleLargeReferencedResource,
		RuleExcessiveBundleSize,
		RuleRepetitiveInstructions,
		RuleUnbalancedSkillContent,
		RuleLargeContent,
		RuleDuplicateHeading,
		RuleMissingWhenToUse,
		RuleMissingNegativeGuidance,
		RuleWeakNegativeGuidance,
		RuleMissingExamples,
		RuleWeakExamples,
		RuleGenericExamples,
		RuleExamplesMissingInvocationPattern,
		RuleAbstractExamples,
		RulePlaceholderHeavyExamples,
		RuleExamplesMissingExpectedOutcome,
		RuleExamplesMissingTriggerInput,
		RuleExampleScopeContradiction,
		RuleExampleGuidanceMismatch,
		RuleIncompleteExample,
		RuleExampleMissingBundleResource,
		RuleLowVarietyExamples,
		RuleMissingUsageGuidance,
		RuleToolSpecificBranding,
		RuleProfileIncompatibleGuidance,
		RuleToolSpecificInstallAssumption,
		RuleNonportableInvocationGuidance,
		RuleGenericProfileToolLocking,
		RuleUnclearToolTargeting,
		RuleAccidentalToolLockIn,
		RuleGenericPortabilityContradiction,
		RuleMixedEcosystemGuidance,
		RuleMissingToolTargetBoundary,
		RuleProfileTargetMismatch,
		RuleExampleEcosystemMismatch,
		RuleSuspiciousRelativePath,
		RuleShortContent,
	}
}
