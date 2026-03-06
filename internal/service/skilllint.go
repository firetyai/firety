package service

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/firety/firety/internal/domain/lint"
	"gopkg.in/yaml.v3"
)

const (
	skillFileName                  = "SKILL.md"
	largeSkillByteThreshold        = 32 * 1024
	shortSkillByteThreshold        = 160
	longFrontMatterNameThreshold   = 60
	shortDescriptionThreshold      = 40
	longDescriptionThreshold       = 220
	weakExamplesRuneThreshold      = 120
	weakNegativeGuidanceThreshold  = 70
	unhelpfulResourceByteThreshold = 24
	placeholderExampleThreshold    = 2
	largeSkillTokenThreshold       = 1800
	excessiveExampleTokenThreshold = 700
	largeResourceTokenThreshold    = 700
	excessiveBundleTokenThreshold  = 3200
	minInstructionTokenThreshold   = 60
	frontMatterOpeningDelimiter    = "---"
	frontMatterClosingDelimiterAlt = "..."
)

var (
	markdownLinkPattern  = regexp.MustCompile(`!?\[[^\]]*]\(([^)]+)\)`)
	codeSpanPattern      = regexp.MustCompile("`([^`\n]+)`")
	yamlErrorLinePattern = regexp.MustCompile(`line (\d+)`)
	plainPathPattern     = regexp.MustCompile(`(?:^|[\s"(])((?:scripts|examples|docs|assets|resources)/[A-Za-z0-9._/\-]+)`)
)

type SkillLinter struct{}

type SkillLintProfile string

const (
	SkillLintProfileGeneric    SkillLintProfile = "generic"
	SkillLintProfileCodex      SkillLintProfile = "codex"
	SkillLintProfileClaudeCode SkillLintProfile = "claude-code"
	SkillLintProfileCopilot    SkillLintProfile = "copilot"
	SkillLintProfileCursor     SkillLintProfile = "cursor"
)

type skillCheck func(report *lint.Report, doc skillDocument, profile SkillLintProfile, strictness lint.Strictness)

var skillChecks = []skillCheck{
	checkFrontMatterIssues,
	checkFrontMatterMetadata,
	checkFrontMatterDescriptionQuality,
	checkFrontMatterBodyConsistency,
	checkTriggerQuality,
	checkTopLevelTitle,
	checkBrokenLocalLinks,
	checkBundleReferences,
	checkMentionedResources,
	checkBundleStructure,
	checkCostEfficiency,
	checkDuplicateHeadings,
	checkWhenToUseGuidance,
	checkNegativeGuidance,
	checkExamplesSection,
	checkExampleRealism,
	checkUsageGuidance,
	checkPortability,
	checkSuspiciousRelativePaths,
	checkLargeContent,
	checkShortContent,
}

type skillDocument struct {
	skillDir      string
	content       string
	body          string
	bodyStartLine int
	headings      []heading
	sections      []section
	bodyLines     []documentLine
	title         heading
	hasTitle      bool
	links         []markdownLink
	mentions      []resourceMention
	hasCodeFence  bool
	bundle        bundleInventory
	frontMatter   frontMatter
}

type documentLine struct {
	Number  int
	Text    string
	Trimmed string
}

type heading struct {
	Level int
	Text  string
	Line  int
}

type markdownLink struct {
	Destination string
	Line        int
}

type resourceMention struct {
	Path string
	Line int
}

type section struct {
	Heading heading
	Lines   []documentLine
}

type bundleInventory struct {
	files             map[string]bundleEntry
	topLevelDirCounts map[string]int
	fileCount         int
}

type bundleEntry struct {
	RelativePath string
	AbsolutePath string
	Size         int64
	IsDir        bool
}

type frontMatter struct {
	Present         bool
	Parsed          bool
	StartLine       int
	RawStartLine    int
	EndLine         int
	HasName         bool
	Name            string
	NameLine        int
	HasDescription  bool
	Description     string
	DescriptionLine int
	Issues          []frontMatterIssue
}

type frontMatterIssue struct {
	Message string
	Line    int
}

type portabilityProfile struct {
	profile            SkillLintProfile
	displayName        string
	brandTerms         []string
	installMarkers     []string
	invocationMarkers  []string
	instructionMarkers []string
}

type portabilitySignal struct {
	profile           SkillLintProfile
	hasBranding       bool
	hasInstallPath    bool
	hasInvocationTerm bool
}

type portabilityAssessment struct {
	declaredTarget         SkillLintProfile
	declaredTargetLine     int
	declaredGeneric        bool
	declaredGenericLine    int
	dominantProfile        SkillLintProfile
	dominantProfileLine    int
	mixedLine              int
	hasBoundaryGuidance    bool
	boundaryLine           int
	exampleMismatchLine    int
	exampleMismatchProfile SkillLintProfile
	profileScores          map[SkillLintProfile]int
}

var portabilityProfiles = []portabilityProfile{
	{
		profile:            SkillLintProfileCodex,
		displayName:        "Codex",
		brandTerms:         []string{"codex", "openai codex"},
		installMarkers:     []string{"$codex_home", ".codex/", "/.codex/", "codex_home/skills"},
		invocationMarkers:  []string{"codex cli", "codex chat"},
		instructionMarkers: []string{"use codex", "install this skill in $codex_home", "place this skill in .codex"},
	},
	{
		profile:            SkillLintProfileClaudeCode,
		displayName:        "Claude Code",
		brandTerms:         []string{"claude code"},
		installMarkers:     []string{".claude/", "/.claude/", ".claude/commands", ".claude/agents"},
		invocationMarkers:  []string{"slash command", "/agents", "/commands"},
		instructionMarkers: []string{"use claude code", "run this as a slash command", "install this under .claude"},
	},
	{
		profile:            SkillLintProfileCopilot,
		displayName:        "GitHub Copilot",
		brandTerms:         []string{"github copilot", "copilot chat", "copilot coding agent"},
		installMarkers:     []string{".github/prompts", ".copilot/"},
		invocationMarkers:  []string{"copilot chat", "chat panel"},
		instructionMarkers: []string{"open copilot chat", "use copilot chat", "install this in .github/prompts"},
	},
	{
		profile:            SkillLintProfileCursor,
		displayName:        "Cursor",
		brandTerms:         []string{"cursor", "cursor rules"},
		installMarkers:     []string{".cursor/", ".cursor/rules"},
		invocationMarkers:  []string{"cmd+k", "composer", "cursor rules"},
		instructionMarkers: []string{"open cursor", "use cursor rules", "press cmd+k"},
	},
}

func NewSkillLinter() SkillLinter {
	return SkillLinter{}
}

func (SkillLinter) Lint(target string) (lint.Report, error) {
	return SkillLinter{}.LintWithProfileAndStrictness(target, SkillLintProfileGeneric, lint.StrictnessDefault)
}

func (SkillLinter) LintWithProfile(target string, profile SkillLintProfile) (lint.Report, error) {
	return SkillLinter{}.LintWithProfileAndStrictness(target, profile, lint.StrictnessDefault)
}

func (SkillLinter) LintWithProfileAndStrictness(target string, profile SkillLintProfile, strictness lint.Strictness) (lint.Report, error) {
	if err := profile.Validate(); err != nil {
		return lint.Report{}, err
	}
	if err := strictness.Validate(); err != nil {
		return lint.Report{}, err
	}

	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return lint.Report{}, err
	}

	report := lint.NewReport(absoluteTarget)

	info, err := os.Stat(absoluteTarget)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.AddError(lint.RuleTargetNotFound, "target path does not exist", absoluteTarget, 0)
			return report, nil
		}

		return lint.Report{}, err
	}

	if !info.IsDir() {
		report.AddError(lint.RuleTargetNotDirectory, "target path is not a directory", absoluteTarget, 0)
		return report, nil
	}

	skillPath := filepath.Join(absoluteTarget, skillFileName)
	content, err := os.ReadFile(skillPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.AddError(lint.RuleMissingSkillMD, "SKILL.md is missing", skillFileName, 0)
			return report, nil
		}

		report.AddError(lint.RuleUnreadableSkillMD, "SKILL.md cannot be read", skillFileName, 0)
		return report, nil
	}

	rawContent := string(content)
	if strings.TrimSpace(rawContent) == "" {
		report.AddError(lint.RuleEmptySkillMD, "SKILL.md is empty", skillFileName, 0)
		return report, nil
	}

	doc := parseSkillDocument(absoluteTarget, rawContent)
	bundle, err := collectBundleInventory(absoluteTarget)
	if err != nil {
		return lint.Report{}, err
	}
	doc.bundle = bundle

	for _, check := range skillChecks {
		check(&report, doc, profile, strictness)
	}
	report.ApplyStrictness(strictness)

	return report, nil
}

func (p SkillLintProfile) Validate() error {
	switch p {
	case SkillLintProfileGeneric, SkillLintProfileCodex, SkillLintProfileClaudeCode, SkillLintProfileCopilot, SkillLintProfileCursor:
		return nil
	default:
		return fmt.Errorf("invalid profile %q: must be one of generic, codex, claude-code, copilot, cursor", p)
	}
}

func ParseSkillLintProfile(raw string) (SkillLintProfile, error) {
	profile := SkillLintProfile(strings.TrimSpace(strings.ToLower(raw)))
	if profile == "" {
		profile = SkillLintProfileGeneric
	}

	if err := profile.Validate(); err != nil {
		return "", err
	}

	return profile, nil
}

func shortDescriptionThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 80
	case lint.StrictnessStrict:
		return 60
	default:
		return shortDescriptionThreshold
	}
}

func longDescriptionThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 160
	case lint.StrictnessStrict:
		return 180
	default:
		return longDescriptionThreshold
	}
}

func weakExamplesRuneThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 240
	case lint.StrictnessStrict:
		return 180
	default:
		return weakExamplesRuneThreshold
	}
}

func weakNegativeGuidanceThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 130
	case lint.StrictnessStrict:
		return 100
	default:
		return weakNegativeGuidanceThreshold
	}
}

func largeSkillTokenThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 1100
	case lint.StrictnessStrict:
		return 1400
	default:
		return largeSkillTokenThreshold
	}
}

func excessiveExampleTokenThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 400
	case lint.StrictnessStrict:
		return 550
	default:
		return excessiveExampleTokenThreshold
	}
}

func largeResourceTokenThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 400
	case lint.StrictnessStrict:
		return 550
	default:
		return largeResourceTokenThreshold
	}
}

func excessiveBundleTokenThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 2000
	case lint.StrictnessStrict:
		return 2600
	default:
		return excessiveBundleTokenThreshold
	}
}

func staleHelperDirCountThresholdFor(strictness lint.Strictness) int {
	switch strictness {
	case lint.StrictnessPedantic:
		return 1
	case lint.StrictnessStrict:
		return 1
	default:
		return 2
	}
}

func parseSkillDocument(skillDir, content string) skillDocument {
	frontMatter, body, bodyStartLine := parseFrontMatter(content)
	headings := make([]heading, 0)
	sections := make([]section, 0)
	bodyLines := make([]documentLine, 0)
	links := make([]markdownLink, 0)
	mentions := make([]resourceMention, 0)
	inFence := false
	hasCodeFence := false
	currentSection := -1
	var title heading
	hasTitle := false

	for index, line := range strings.Split(body, "\n") {
		lineNumber := bodyStartLine + index
		trimmed := strings.TrimSpace(line)

		if isFenceLine(trimmed) {
			inFence = !inFence
			hasCodeFence = true
			continue
		}

		if inFence {
			continue
		}

		bodyLine := documentLine{
			Number:  lineNumber,
			Text:    line,
			Trimmed: trimmed,
		}
		bodyLines = append(bodyLines, bodyLine)

		if parsedHeading, ok := parseHeading(lineNumber, trimmed); ok {
			headings = append(headings, parsedHeading)
			sections = append(sections, section{Heading: parsedHeading})
			currentSection = len(sections) - 1

			if parsedHeading.Level == 1 && !hasTitle {
				title = parsedHeading
				hasTitle = true
			}
		} else if currentSection >= 0 && trimmed != "" {
			sections[currentSection].Lines = append(sections[currentSection].Lines, bodyLine)
		}

		for _, match := range markdownLinkPattern.FindAllStringSubmatch(line, -1) {
			if len(match) < 2 {
				continue
			}

			destination := extractLinkDestination(match[1])
			if destination == "" {
				continue
			}

			links = append(links, markdownLink{
				Destination: destination,
				Line:        lineNumber,
			})
		}

		for _, match := range codeSpanPattern.FindAllStringSubmatch(line, -1) {
			if len(match) < 2 {
				continue
			}

			candidate := strings.TrimSpace(match[1])
			if !looksLikeLocalResourceMention(candidate) {
				continue
			}

			mentions = append(mentions, resourceMention{
				Path: candidate,
				Line: lineNumber,
			})
		}
	}

	return skillDocument{
		skillDir:      skillDir,
		content:       content,
		body:          body,
		bodyStartLine: bodyStartLine,
		headings:      headings,
		sections:      sections,
		bodyLines:     bodyLines,
		title:         title,
		hasTitle:      hasTitle,
		links:         links,
		mentions:      mentions,
		hasCodeFence:  hasCodeFence,
		frontMatter:   frontMatter,
	}
}

func collectBundleInventory(skillDir string) (bundleInventory, error) {
	inventory := bundleInventory{
		files:             make(map[string]bundleEntry),
		topLevelDirCounts: make(map[string]int),
	}

	err := filepath.WalkDir(skillDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(skillDir, path)
		if err != nil {
			return err
		}

		relativePath = filepath.ToSlash(relativePath)
		if relativePath == "." {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		bundleEntry := bundleEntry{
			RelativePath: relativePath,
			AbsolutePath: path,
			Size:         info.Size(),
			IsDir:        entry.IsDir(),
		}
		inventory.files[relativePath] = bundleEntry

		if entry.IsDir() {
			return nil
		}

		inventory.fileCount++
		topLevelDir := relativePath
		if slashIndex := strings.Index(topLevelDir, "/"); slashIndex >= 0 {
			topLevelDir = topLevelDir[:slashIndex]
		}
		inventory.topLevelDirCounts[topLevelDir]++

		return nil
	})
	if err != nil {
		return bundleInventory{}, err
	}

	return inventory, nil
}

func parseFrontMatter(content string) (frontMatter, string, int) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontMatterOpeningDelimiter {
		return frontMatter{}, content, 1
	}

	fm := frontMatter{
		Present:      true,
		StartLine:    1,
		RawStartLine: 2,
	}

	closingIndex := -1
	for index := 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == frontMatterOpeningDelimiter || trimmed == frontMatterClosingDelimiterAlt {
			closingIndex = index
			break
		}
	}

	if closingIndex == -1 {
		fm.Issues = append(fm.Issues, frontMatterIssue{
			Message: "front matter is missing a closing delimiter",
			Line:    fm.StartLine,
		})
		return fm, strings.Join(lines[1:], "\n"), 2
	}

	fm.EndLine = closingIndex + 1
	bodyStartLine := closingIndex + 2
	body := strings.Join(lines[closingIndex+1:], "\n")
	raw := strings.Join(lines[1:closingIndex], "\n")

	if strings.TrimSpace(raw) == "" {
		fm.Parsed = true
		return fm, body, bodyStartLine
	}

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		fm.Issues = append(fm.Issues, frontMatterIssue{
			Message: fmt.Sprintf("front matter is invalid: %v", err),
			Line:    yamlErrorLineFromMessage(fm.RawStartLine, err),
		})
		return fm, body, bodyStartLine
	}

	if len(node.Content) == 0 {
		fm.Parsed = true
		return fm, body, bodyStartLine
	}

	topLevel := node.Content[0]
	if topLevel.Kind != yaml.MappingNode {
		fm.Issues = append(fm.Issues, frontMatterIssue{
			Message: "front matter must be a YAML mapping",
			Line:    fm.RawStartLine,
		})
		return fm, body, bodyStartLine
	}

	fm.Parsed = true
	seenKeys := make(map[string]int, len(topLevel.Content)/2)

	for index := 0; index+1 < len(topLevel.Content); index += 2 {
		keyNode := topLevel.Content[index]
		valueNode := topLevel.Content[index+1]
		keyLine := yamlNodeLine(fm.RawStartLine, keyNode.Line)

		if keyNode.Kind != yaml.ScalarNode {
			fm.Issues = append(fm.Issues, frontMatterIssue{
				Message: "front matter keys must be plain strings",
				Line:    keyLine,
			})
			continue
		}

		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			fm.Issues = append(fm.Issues, frontMatterIssue{
				Message: "front matter contains an empty key",
				Line:    keyLine,
			})
			continue
		}

		if originalLine, exists := seenKeys[key]; exists {
			fm.Issues = append(fm.Issues, frontMatterIssue{
				Message: fmt.Sprintf("front matter key %q is duplicated (first defined at line %d)", key, originalLine),
				Line:    keyLine,
			})
			continue
		}
		seenKeys[key] = keyLine

		switch key {
		case "name":
			fm.HasName = true
			fm.NameLine = keyLine
			if valueNode.Kind != yaml.ScalarNode {
				fm.Issues = append(fm.Issues, frontMatterIssue{
					Message: `front matter field "name" must be a string`,
					Line:    keyLine,
				})
				continue
			}
			fm.Name = strings.TrimSpace(valueNode.Value)
		case "description":
			fm.HasDescription = true
			fm.DescriptionLine = keyLine
			if valueNode.Kind != yaml.ScalarNode {
				fm.Issues = append(fm.Issues, frontMatterIssue{
					Message: `front matter field "description" must be a string`,
					Line:    keyLine,
				})
				continue
			}
			fm.Description = strings.TrimSpace(valueNode.Value)
		}
	}

	return fm, body, bodyStartLine
}

func parseHeading(lineNumber int, line string) (heading, bool) {
	if line == "" || !strings.HasPrefix(line, "#") {
		return heading{}, false
	}

	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}

	if level == 0 || level >= len(line) || line[level] != ' ' {
		return heading{}, false
	}

	text := strings.TrimSpace(line[level:])
	if text == "" {
		return heading{}, false
	}

	return heading{
		Level: level,
		Text:  text,
		Line:  lineNumber,
	}, true
}

func isFenceLine(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func extractLinkDestination(raw string) string {
	destination := strings.TrimSpace(raw)
	if destination == "" {
		return ""
	}

	if strings.HasPrefix(destination, "<") {
		end := strings.Index(destination, ">")
		if end > 1 {
			return destination[1:end]
		}
	}

	if firstSpace := strings.IndexAny(destination, " \t"); firstSpace >= 0 {
		return destination[:firstSpace]
	}

	return destination
}

func checkFrontMatterIssues(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	for _, issue := range doc.frontMatter.Issues {
		report.AddError(lint.RuleInvalidFrontMatter, issue.Message, skillFileName, issue.Line)
	}
}

func checkFrontMatterMetadata(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if !doc.frontMatter.Present || !doc.frontMatter.Parsed {
		return
	}

	if !doc.frontMatter.HasName {
		report.AddError(lint.RuleMissingFrontMatterName, `front matter is missing required field "name"`, skillFileName, doc.frontMatter.RawStartLine)
	} else if doc.frontMatter.Name == "" {
		report.AddError(lint.RuleEmptyFrontMatterName, `front matter field "name" is empty`, skillFileName, doc.frontMatter.NameLine)
	} else if len([]rune(doc.frontMatter.Name)) > longFrontMatterNameThreshold {
		report.AddWarning(lint.RuleLongFrontMatterName, `front matter field "name" is unusually long`, skillFileName, doc.frontMatter.NameLine)
	}

	if !doc.frontMatter.HasDescription {
		report.AddWarning(lint.RuleMissingFrontMatterDescription, `front matter is missing recommended field "description"`, skillFileName, doc.frontMatter.RawStartLine)
		return
	}

	if doc.frontMatter.Description == "" {
		report.AddWarning(lint.RuleEmptyFrontMatterDescription, `front matter field "description" is empty`, skillFileName, doc.frontMatter.DescriptionLine)
	}
}

func checkFrontMatterDescriptionQuality(report *lint.Report, doc skillDocument, _ SkillLintProfile, strictness lint.Strictness) {
	if !doc.frontMatter.Present || !doc.frontMatter.Parsed || !doc.frontMatter.HasDescription {
		return
	}

	description := doc.frontMatter.Description
	if description == "" {
		return
	}

	line := doc.frontMatter.DescriptionLine
	if len([]rune(description)) < shortDescriptionThresholdFor(strictness) {
		report.AddWarning(lint.RuleShortFrontMatterDescription, `front matter field "description" is too short to be useful`, skillFileName, line)
	}

	if len([]rune(description)) > longDescriptionThresholdFor(strictness) {
		report.AddWarning(lint.RuleLongFrontMatterDescription, `front matter field "description" is excessively long`, skillFileName, line)
	}

	if isVagueDescription(description) {
		report.AddWarning(lint.RuleVagueDescription, `front matter field "description" is too vague to clearly distinguish the skill`, skillFileName, line)
	}
}

func checkFrontMatterBodyConsistency(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if !doc.frontMatter.Present || !doc.frontMatter.Parsed {
		return
	}

	if doc.frontMatter.Name != "" && doc.hasTitle && hasNameTitleMismatch(doc.frontMatter.Name, doc.title.Text) {
		report.AddWarning(lint.RuleNameTitleMismatch, `front matter field "name" appears inconsistent with the top-level title`, skillFileName, doc.title.Line)
	}

	if doc.frontMatter.Description != "" {
		if hasDescriptionBodyMismatch(doc.frontMatter.Description, doc) {
			report.AddWarning(lint.RuleDescriptionBodyMismatch, `front matter field "description" appears inconsistent with the document body`, skillFileName, doc.frontMatter.DescriptionLine)
		}

		if line, ok := findScopeMismatchLine(doc.frontMatter.Description, doc); ok {
			report.AddWarning(lint.RuleScopeMismatch, "document body describes a broader or different scope than the front matter suggests", skillFileName, line)
		}
	}
}

func checkTriggerQuality(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if line, ok := genericSkillNameLine(doc); ok {
		report.AddWarning(lint.RuleGenericName, "skill name is too generic to help an agent distinguish when to use it", skillFileName, line)
	}

	if line, ok := genericTriggerDescriptionLine(doc); ok {
		report.AddWarning(lint.RuleGenericTriggerDescription, "description is too generic to make this skill stand out from normal assistant behavior", skillFileName, line)
	}

	if line, ok := diffuseScopeLine(doc); ok {
		report.AddWarning(lint.RuleDiffuseScope, "skill appears to cover too many loosely-related tasks to trigger cleanly", skillFileName, line)
	}

	if section, ok := findSection(doc, isWhenToUseHeading); ok {
		whenText := joinSectionText(section)
		if isOverbroadWhenToUseText(whenText) {
			report.AddWarning(lint.RuleOverbroadWhenToUse, "when-to-use guidance is broad enough that many generic requests could match it", skillFileName, section.Heading.Line)
		}
	}

	if line, ok := lowDistinctivenessLine(doc); ok {
		report.AddWarning(lint.RuleLowDistinctiveness, "skill language lacks distinctive trigger terms that would help routing", skillFileName, line)
	}

	if line, ok := exampleTriggerMismatchLine(doc); ok {
		report.AddWarning(lint.RuleExampleTriggerMismatch, "examples point toward a different or weaker trigger concept than the main description", skillFileName, line)
	}

	if line, ok := weakTriggerPatternLine(doc); ok {
		report.AddWarning(lint.RuleWeakTriggerPattern, "examples do not reinforce a clear and repeated trigger pattern", skillFileName, line)
	}

	if line, ok := triggerScopeInconsistencyLine(doc); ok {
		report.AddWarning(lint.RuleTriggerScopeInconsistency, "name, description, and body suggest different trigger concepts", skillFileName, line)
	}
}

func checkTopLevelTitle(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if doc.hasTitle {
		return
	}

	report.AddError(lint.RuleMissingTitle, "SKILL.md is missing a top-level markdown title", skillFileName, doc.bodyStartLine)
}

func checkBrokenLocalLinks(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	for _, link := range doc.links {
		if !isLocalLink(link.Destination) {
			continue
		}

		targetPath := cleanLinkPath(link.Destination)
		resolvedPath := resolveLinkPath(doc.skillDir, targetPath)

		if _, err := os.Stat(resolvedPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				report.AddError(lint.RuleBrokenLocalLink, "local markdown link points to a missing file", targetPath, link.Line)
				continue
			}

			report.AddError(lint.RuleBrokenLocalLink, "local markdown link could not be resolved", targetPath, link.Line)
		}
	}
}

func checkBundleReferences(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	referenceCounts := make(map[string]int)

	for _, link := range doc.links {
		if !isLocalLink(link.Destination) {
			continue
		}

		targetPath := cleanLinkPath(link.Destination)
		if targetPath == "." {
			continue
		}

		resolvedPath := resolveLinkPath(doc.skillDir, targetPath)
		if !isWithinSkillRoot(doc.skillDir, resolvedPath) {
			report.AddWarning(lint.RuleReferenceOutsideRoot, "referenced resource escapes the skill root", targetPath, link.Line)
		}

		entry, ok := lookupBundleEntry(doc.bundle, doc.skillDir, targetPath)
		if !ok {
			continue
		}

		if entry.IsDir {
			report.AddWarning(lint.RuleReferencedDirectoryInsteadOfFile, "referenced resource is a directory where a file is expected", targetPath, link.Line)
			continue
		}

		if isSuspiciousReferencedResource(targetPath) {
			report.AddWarning(lint.RuleSuspiciousReferencedResource, "referenced resource has a suspicious binary or package-like extension", targetPath, link.Line)
		}

		if isEmptyReferencedResource(targetPath, entry.Size) {
			report.AddWarning(lint.RuleEmptyReferencedResource, "referenced resource exists but is empty", targetPath, link.Line)
		} else if isUnhelpfulReferencedResource(targetPath, entry.Size) {
			report.AddWarning(lint.RuleUnhelpfulReferencedResource, "referenced resource exists but appears too short to be useful", targetPath, link.Line)
		}

		normalizedPath := filepath.ToSlash(targetPath)
		referenceCounts[normalizedPath]++
		if referenceCounts[normalizedPath] == 3 {
			report.AddWarning(lint.RuleDuplicateResourceReference, "the same local resource is referenced repeatedly", targetPath, link.Line)
		}
	}
}

func checkMentionedResources(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	linkedPaths := make(map[string]struct{}, len(doc.links))
	for _, link := range doc.links {
		if !isLocalLink(link.Destination) {
			continue
		}

		linkedPaths[filepath.ToSlash(cleanLinkPath(link.Destination))] = struct{}{}
	}

	for _, mention := range doc.mentions {
		normalizedPath := filepath.ToSlash(cleanLinkPath(mention.Path))
		if normalizedPath == "." {
			continue
		}

		if _, linked := linkedPaths[normalizedPath]; linked {
			continue
		}

		resolvedPath := resolveLinkPath(doc.skillDir, normalizedPath)
		if !isWithinSkillRoot(doc.skillDir, resolvedPath) {
			report.AddWarning(lint.RuleReferenceOutsideRoot, "mentioned resource escapes the skill root", normalizedPath, mention.Line)
			continue
		}

		if _, ok := lookupBundleEntry(doc.bundle, doc.skillDir, normalizedPath); !ok {
			report.AddWarning(lint.RuleMissingMentionedResource, "mentioned local resource is missing from the bundle", normalizedPath, mention.Line)
		}
	}
}

func checkBundleStructure(report *lint.Report, doc skillDocument, _ SkillLintProfile, strictness lint.Strictness) {
	if doc.bundle.fileCount <= 1 && hasStrongBundleExpectation(doc) {
		report.AddWarning(lint.RuleInconsistentBundleStructure, "the bundle contains almost no supporting files despite documented resource references", skillFileName, 0)
	}

	referencedDirs := referencedTopLevelDirs(doc)
	staleHelperThreshold := staleHelperDirCountThresholdFor(strictness)
	for _, helperDir := range []string{"scripts", "examples", "assets", "docs"} {
		if doc.bundle.topLevelDirCounts[helperDir] >= staleHelperThreshold {
			if _, referenced := referencedDirs[helperDir]; !referenced {
				report.AddWarning(lint.RulePossiblyStaleResource, fmt.Sprintf("bundle contains %s/ resources that are never referenced from SKILL.md", helperDir), helperDir, 0)
				return
			}
		}
	}
}

func checkCostEfficiency(report *lint.Report, doc skillDocument, _ SkillLintProfile, strictness lint.Strictness) {
	skillTokens := estimateTokenCount(doc.content)
	if skillTokens >= largeSkillTokenThresholdFor(strictness) {
		report.AddWarning(
			lint.RuleLargeSkillMD,
			fmt.Sprintf("SKILL.md is estimated at about %d tokens, which is likely heavier than necessary for a single skill", skillTokens),
			skillFileName,
			0,
		)
	}

	if section, ok := findSection(doc, func(heading heading) bool {
		return heading.Level > 1 && strings.Contains(normalizeHeading(heading.Text), "example")
	}); ok {
		exampleText := joinSectionText(section)
		exampleTokens := estimateTokenCount(exampleText)
		instructionTokens := estimateInstructionTokens(doc)

		if exampleTokens >= excessiveExampleTokenThresholdFor(strictness) || (exampleTokens >= 420 && exampleTokens*100 >= skillTokens*45) {
			report.AddWarning(
				lint.RuleExcessiveExampleVolume,
				fmt.Sprintf("examples section is estimated at about %d tokens, which may dominate the skill's context cost", exampleTokens),
				skillFileName,
				section.Heading.Line,
			)
		}

		if hasDuplicateExamples(section) {
			report.AddWarning(
				lint.RuleDuplicateExamples,
				"examples section appears to repeat the same scenario with minimal variation",
				skillFileName,
				section.Heading.Line,
			)
		}

		if instructionTokens >= minInstructionTokenThreshold {
			if exampleTokens >= instructionTokens*3 && exampleTokens >= 450 {
				report.AddWarning(
					lint.RuleUnbalancedSkillContent,
					fmt.Sprintf("examples are estimated at about %d tokens versus %d tokens of instructions, which may be heavier than necessary", exampleTokens, instructionTokens),
					skillFileName,
					section.Heading.Line,
				)
			}

			if instructionTokens >= 900 && exampleTokens <= 120 {
				report.AddWarning(
					lint.RuleUnbalancedSkillContent,
					fmt.Sprintf("instructions are estimated at about %d tokens versus only %d tokens of examples, which may make the skill harder to apply quickly", instructionTokens, exampleTokens),
					skillFileName,
					section.Heading.Line,
				)
			}
		}
	}

	for _, section := range doc.sections {
		if section.Heading.Level <= 1 {
			continue
		}

		normalizedHeading := normalizeHeading(section.Heading.Text)
		if strings.Contains(normalizedHeading, "example") {
			continue
		}

		if isRepetitiveInstructionSection(section) {
			report.AddWarning(
				lint.RuleRepetitiveInstructions,
				"section appears to repeat the same instruction phrasing several times without adding much guidance",
				skillFileName,
				section.Heading.Line,
			)
			break
		}
	}

	likelyLoadedTokens := skillTokens
	reportedResources := make(map[string]struct{})
	for _, link := range doc.links {
		if !isLocalLink(link.Destination) {
			continue
		}

		targetPath := filepath.ToSlash(cleanLinkPath(link.Destination))
		entry, ok := lookupBundleEntry(doc.bundle, doc.skillDir, targetPath)
		if !ok || entry.IsDir {
			continue
		}

		extension := strings.ToLower(filepath.Ext(targetPath))
		if !isTextLikeResource(extension) && !isScriptLikeResource(extension) {
			continue
		}

		resourceTokens := estimateTokenCountFromBytes(entry.Size)
		likelyLoadedTokens += resourceTokens

		if resourceTokens >= largeResourceTokenThresholdFor(strictness) {
			if _, seen := reportedResources[targetPath]; !seen {
				report.AddWarning(
					lint.RuleLargeReferencedResource,
					fmt.Sprintf("referenced resource is estimated at about %d tokens, which may be costly to load alongside SKILL.md", resourceTokens),
					targetPath,
					link.Line,
				)
				reportedResources[targetPath] = struct{}{}
			}
		}
	}

	if likelyLoadedTokens >= excessiveBundleTokenThresholdFor(strictness) {
		report.AddWarning(
			lint.RuleExcessiveBundleSize,
			fmt.Sprintf("likely-loaded bundle content is estimated at about %d tokens, which may be expensive for a single skill", likelyLoadedTokens),
			skillFileName,
			0,
		)
	}
}

func checkDuplicateHeadings(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	seen := make(map[string]int, len(doc.headings))

	for _, heading := range doc.headings {
		key := normalizeHeading(heading.Text)
		if key == "" {
			continue
		}

		if _, exists := seen[key]; exists {
			report.AddWarning(lint.RuleDuplicateHeading, "duplicate heading found", skillFileName, heading.Line)
			continue
		}

		seen[key] = heading.Line
	}
}

func checkWhenToUseGuidance(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if hasWhenToUseGuidance(doc) {
		return
	}

	report.AddWarning(lint.RuleMissingWhenToUse, "no obvious guidance explains when to use this skill", skillFileName, 0)
}

func checkNegativeGuidance(report *lint.Report, doc skillDocument, _ SkillLintProfile, strictness lint.Strictness) {
	if section, ok := findSection(doc, isNegativeGuidanceHeading); ok {
		if isWeakNegativeGuidanceText(joinSectionText(section), strictness) {
			report.AddWarning(lint.RuleWeakNegativeGuidance, "negative guidance exists but appears too weak or generic", skillFileName, section.Heading.Line)
		}
		return
	}

	line, text, ok := findBodyLine(doc, negativeGuidancePhrases)
	if !ok {
		report.AddWarning(lint.RuleMissingNegativeGuidance, "no obvious guidance explains when not to use this skill or where its boundaries are", skillFileName, 0)
		return
	}

	if isWeakNegativeGuidanceText(text, strictness) {
		report.AddWarning(lint.RuleWeakNegativeGuidance, "negative guidance exists but appears too weak or generic", skillFileName, line)
	}
}

func checkExamplesSection(report *lint.Report, doc skillDocument, _ SkillLintProfile, strictness lint.Strictness) {
	examples := inspectExamples(doc, strictness)
	if !examples.Present {
		report.AddWarning(lint.RuleMissingExamples, "no obvious examples section or usage examples found", skillFileName, 0)
		return
	}

	if examples.Weak {
		report.AddWarning(lint.RuleWeakExamples, "examples exist but appear too short or content-light", skillFileName, examples.Line)
	}

	if examples.Generic {
		report.AddWarning(lint.RuleGenericExamples, "examples exist but appear too generic", skillFileName, examples.Line)
	}

	if !examples.HasInvocationPattern {
		report.AddWarning(lint.RuleExamplesMissingInvocationPattern, "examples do not show a clear invocation or request pattern", skillFileName, examples.Line)
	}
}

func checkExampleRealism(report *lint.Report, doc skillDocument, _ SkillLintProfile, strictness lint.Strictness) {
	examples := inspectExamples(doc, strictness)
	if !examples.Present {
		return
	}

	if examples.AbstractLine > 0 {
		report.AddWarning(lint.RuleAbstractExamples, "examples look too abstract to show a realistic request or workflow", skillFileName, examples.AbstractLine)
	}

	if examples.PlaceholderLine > 0 {
		report.AddWarning(lint.RulePlaceholderHeavyExamples, "examples rely on placeholders instead of enough concrete values to be realistically reusable", skillFileName, examples.PlaceholderLine)
	}

	if examples.MissingOutcomeLine > 0 {
		report.AddWarning(lint.RuleExamplesMissingExpectedOutcome, "examples show how to trigger the skill but not the expected result or output shape", skillFileName, examples.MissingOutcomeLine)
	}

	if examples.MissingTriggerLine > 0 {
		report.AddWarning(lint.RuleExamplesMissingTriggerInput, "examples show outcomes without a clear triggering request, input, or invocation", skillFileName, examples.MissingTriggerLine)
	}

	if examples.ScopeContradictionLine > 0 {
		report.AddWarning(lint.RuleExampleScopeContradiction, "an example appears to contradict the documented limitations or supported scope", skillFileName, examples.ScopeContradictionLine)
	}

	if examples.GuidanceMismatchLine > 0 {
		report.AddWarning(lint.RuleExampleGuidanceMismatch, "examples do not appear to match the documented when-to-use guidance", skillFileName, examples.GuidanceMismatchLine)
	}

	if examples.IncompleteLine > 0 {
		report.AddWarning(lint.RuleIncompleteExample, "an example appears incomplete, truncated, or left with placeholder follow-up text", skillFileName, examples.IncompleteLine)
	}

	if examples.MissingResourceLine > 0 {
		report.AddWarning(
			lint.RuleExampleMissingBundleResource,
			fmt.Sprintf("example references local resource %q, but it is missing from the bundle", examples.MissingResourcePath),
			examples.MissingResourcePath,
			examples.MissingResourceLine,
		)
	}

	if examples.LowVarietyLine > 0 {
		report.AddWarning(lint.RuleLowVarietyExamples, "examples are too similar to demonstrate varied realistic usage patterns", skillFileName, examples.LowVarietyLine)
	}
}

func checkUsageGuidance(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if hasInvocationGuidance(doc) {
		return
	}

	report.AddWarning(lint.RuleMissingUsageGuidance, "no obvious invocation guidance or input expectations found", skillFileName, 0)
}

func checkPortability(report *lint.Report, doc skillDocument, profile SkillLintProfile, _ lint.Strictness) {
	assessment := analyzePortability(doc)

	if assessment.mixedLine > 0 {
		report.AddWarning(lint.RuleMixedEcosystemGuidance, "guidance mixes multiple tool ecosystems in a way that is likely to confuse users or routing", skillFileName, assessment.mixedLine)
	}

	if assessment.exampleMismatchLine > 0 {
		report.AddWarning(
			lint.RuleExampleEcosystemMismatch,
			fmt.Sprintf("examples reinforce %s conventions instead of the skill's apparent intended ecosystem", profileDisplayName(assessment.exampleMismatchProfile)),
			skillFileName,
			assessment.exampleMismatchLine,
		)
	}

	if profile == SkillLintProfileGeneric {
		if assessment.declaredGeneric && assessment.dominantProfile != "" && !assessment.isClearlyTargeted() {
			report.AddWarning(
				lint.RuleGenericPortabilityContradiction,
				fmt.Sprintf("skill describes itself as generic or portable but repeatedly relies on %s-specific conventions", profileDisplayName(assessment.dominantProfile)),
				skillFileName,
				portabilityPrimaryLine(assessment),
			)
			report.AddWarning(
				lint.RuleAccidentalToolLockIn,
				fmt.Sprintf("skill appears unintentionally locked to %s conventions despite claiming broader portability", profileDisplayName(assessment.dominantProfile)),
				skillFileName,
				portabilityPrimaryLine(assessment),
			)
		} else if assessment.dominantProfile != "" && !assessment.isClearlyTargeted() {
			report.AddWarning(
				lint.RuleAccidentalToolLockIn,
				fmt.Sprintf("skill appears unintentionally locked to %s conventions instead of staying broadly portable", profileDisplayName(assessment.dominantProfile)),
				skillFileName,
				portabilityPrimaryLine(assessment),
			)
			report.AddWarning(
				lint.RuleGenericProfileToolLocking,
				fmt.Sprintf("the skill appears too tightly coupled to %s for the generic profile", profileDisplayName(assessment.dominantProfile)),
				skillFileName,
				assessment.dominantProfileLine,
			)
		}
	}

	if assessment.dominantProfile != "" && !assessment.isClearlyTargeted() && !assessment.declaredGeneric {
		report.AddWarning(
			lint.RuleUnclearToolTargeting,
			fmt.Sprintf("skill uses strong %s-specific conventions without clearly stating that target in its name, description, or guidance", profileDisplayName(assessment.dominantProfile)),
			skillFileName,
			assessment.dominantProfileLine,
		)
	}

	if assessment.isClearlyTargeted() && !assessment.hasBoundaryGuidance {
		report.AddWarning(
			lint.RuleMissingToolTargetBoundary,
			fmt.Sprintf("skill appears intentionally targeted at %s but does not explain its boundaries or intended audience clearly", profileDisplayName(assessment.declaredTarget)),
			skillFileName,
			portabilityPrimaryLine(assessment),
		)
	}

	if profile != SkillLintProfileGeneric && profileStronglyDisagrees(profile, assessment) {
		report.AddWarning(
			lint.RuleProfileTargetMismatch,
			fmt.Sprintf("selected %s profile conflicts with the skill's apparent %s targeting", profileDisplayName(profile), profileDisplayName(portabilityExpectedProfile(assessment))),
			skillFileName,
			portabilityPrimaryLine(assessment),
		)
	}

	for _, line := range portabilityLines(doc) {
		if line.Trimmed == "" {
			continue
		}

		signals := collectPortabilitySignals(line.Trimmed)
		if len(signals) == 0 {
			continue
		}

		if lineContainsMixedProfiles(signals) && lineContainsInstructionSignal(line.Trimmed) {
			continue
		}

		if profile == SkillLintProfileGeneric {
			if assessment.isHonestTargetedLine(signals) {
				continue
			}

			for _, signal := range signals {
				if signal.hasInstallPath {
					report.AddWarning(lint.RuleToolSpecificInstallAssumption, "instructions assume a tool-specific install location or filesystem layout", skillFileName, line.Number)
				}

				if signal.hasInvocationTerm {
					report.AddWarning(lint.RuleNonportableInvocationGuidance, "invocation guidance depends on tool-specific commands or UX conventions", skillFileName, line.Number)
				}

				if signal.hasBranding && hasStrongBrandingLock(signal, signals) {
					report.AddWarning(lint.RuleToolSpecificBranding, fmt.Sprintf("instructions are strongly branded around %s instead of using portable wording", profileDisplayName(signal.profile)), skillFileName, line.Number)
				}
			}

			continue
		}

		for _, signal := range signals {
			if signal.profile == profile && assessment.isClearlyTargetedAt(profile) {
				continue
			}

			if signal.profile == profile {
				continue
			}

			if signal.hasInstallPath {
				report.AddWarning(lint.RuleToolSpecificInstallAssumption, fmt.Sprintf("instructions assume a %s-specific install location or filesystem layout", profileDisplayName(signal.profile)), skillFileName, line.Number)
				continue
			}

			if signal.hasInvocationTerm || lineContainsInstructionSignal(line.Trimmed) {
				report.AddWarning(lint.RuleProfileIncompatibleGuidance, fmt.Sprintf("guidance appears tailored to %s rather than the selected %s profile", profileDisplayName(signal.profile), profileDisplayName(profile)), skillFileName, line.Number)
				continue
			}

			if signal.hasBranding {
				report.AddWarning(lint.RuleToolSpecificBranding, fmt.Sprintf("instructions reference %s branding that may reduce portability for the selected %s profile", profileDisplayName(signal.profile), profileDisplayName(profile)), skillFileName, line.Number)
			}
		}
	}
}

func checkSuspiciousRelativePaths(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	for _, link := range doc.links {
		if !isLocalLink(link.Destination) {
			continue
		}

		targetPath := cleanLinkPath(link.Destination)
		normalized := filepath.ToSlash(targetPath)

		if strings.HasPrefix(normalized, "../") {
			report.AddWarning(lint.RuleSuspiciousRelativePath, "relative path escapes the skill directory", targetPath, link.Line)
		}

		if strings.Contains(targetPath, `\`) {
			report.AddWarning(lint.RuleSuspiciousRelativePath, "relative path uses backslashes", targetPath, link.Line)
		}
	}
}

func checkLargeContent(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if len(doc.content) > largeSkillByteThreshold {
		report.AddWarning(lint.RuleLargeContent, "SKILL.md is very large and may be hard to maintain", skillFileName, 0)
	}
}

func checkShortContent(report *lint.Report, doc skillDocument, _ SkillLintProfile, _ lint.Strictness) {
	if len(strings.TrimSpace(doc.content)) < shortSkillByteThreshold {
		report.AddWarning(lint.RuleShortContent, "SKILL.md content is very short and may not be useful", skillFileName, 0)
	}
}

func hasWhenToUseGuidance(doc skillDocument) bool {
	if _, ok := findSection(doc, isWhenToUseHeading); ok {
		return true
	}

	lowerBody := strings.ToLower(doc.body)
	for _, phrase := range []string{
		"use this skill when",
		"use when",
		"when to use",
		"best for",
		"ideal for",
		"works best when",
		"choose this skill when",
		"good fit when",
	} {
		if strings.Contains(lowerBody, phrase) {
			return true
		}
	}

	return false
}

func isWhenToUseHeading(heading heading) bool {
	normalized := normalizeHeading(heading.Text)
	return strings.Contains(normalized, "when to use") ||
		strings.Contains(normalized, "use cases") ||
		strings.Contains(normalized, "best for")
}

type exampleInspection struct {
	Present                bool
	Weak                   bool
	Generic                bool
	HasInvocationPattern   bool
	HasTriggerInput        bool
	HasExpectedOutcome     bool
	AbstractLine           int
	PlaceholderLine        int
	MissingOutcomeLine     int
	MissingTriggerLine     int
	ScopeContradictionLine int
	GuidanceMismatchLine   int
	IncompleteLine         int
	MissingResourcePath    string
	MissingResourceLine    int
	LowVarietyLine         int
	Line                   int
}

func inspectExamples(doc skillDocument, strictness lint.Strictness) exampleInspection {
	if section, ok := findSection(doc, func(heading heading) bool {
		return heading.Level > 1 && strings.Contains(normalizeHeading(heading.Text), "example")
	}); ok {
		text := joinSectionText(section)
		inspection := exampleInspection{
			Present:              true,
			Weak:                 isWeakExamplesSection(section, strictness),
			Generic:              isGenericExamplesText(text),
			HasInvocationPattern: hasExampleInvocationPattern(text),
			Line:                 section.Heading.Line,
		}
		inspection.HasTriggerInput = hasExampleTriggerInput(section)
		inspection.HasExpectedOutcome = hasExampleExpectedOutcome(section)
		inspection.AbstractLine = abstractExampleLine(section)
		inspection.PlaceholderLine = placeholderHeavyExampleLine(section)
		inspection.MissingOutcomeLine = missingExampleOutcomeLine(section, inspection)
		inspection.MissingTriggerLine = missingExampleTriggerLine(section, inspection)
		inspection.ScopeContradictionLine = exampleScopeContradictionLine(doc, section)
		inspection.GuidanceMismatchLine = exampleGuidanceMismatchLine(doc, section, inspection)
		inspection.IncompleteLine = incompleteExampleLine(section)
		inspection.MissingResourcePath, inspection.MissingResourceLine = missingExampleBundleResource(doc, section)
		inspection.LowVarietyLine = lowVarietyExampleLine(section)
		return inspection
	}

	if doc.hasCodeFence {
		text := lowerJoinedBodyText(doc.bodyLines)
		return exampleInspection{
			Present:              true,
			Weak:                 false,
			Generic:              false,
			HasInvocationPattern: hasExampleInvocationPattern(text),
			Line:                 0,
		}
	}

	lowerBody := strings.ToLower(doc.body)
	if strings.Contains(lowerBody, "for example") ||
		strings.Contains(lowerBody, "example:") ||
		(strings.Contains(lowerBody, "input") && strings.Contains(lowerBody, "output")) {
		return exampleInspection{
			Present:              true,
			Weak:                 true,
			Generic:              isGenericExamplesText(lowerBody),
			HasInvocationPattern: hasExampleInvocationPattern(lowerBody),
			Line:                 0,
		}
	}

	return exampleInspection{}
}

func hasInvocationGuidance(doc skillDocument) bool {
	for _, heading := range doc.headings {
		normalized := normalizeHeading(heading.Text)
		if strings.Contains(normalized, "usage") ||
			strings.Contains(normalized, "invoke") ||
			strings.Contains(normalized, "invocation") ||
			strings.Contains(normalized, "input") ||
			strings.Contains(normalized, "inputs") ||
			strings.Contains(normalized, "arguments") ||
			strings.Contains(normalized, "parameters") ||
			strings.Contains(normalized, "getting started") ||
			strings.Contains(normalized, "quickstart") {
			return true
		}
	}

	lowerBody := strings.ToLower(doc.body)
	for _, phrase := range []string{
		"run `",
		"invoke `",
		"use `",
		"command `",
		"input:",
		"inputs:",
		"expects ",
		"arguments:",
		"parameters:",
		"pass ",
	} {
		if strings.Contains(lowerBody, phrase) {
			return true
		}
	}

	return false
}

func isVagueDescription(description string) bool {
	lowerDescription := strings.ToLower(strings.TrimSpace(description))
	if lowerDescription == "" {
		return false
	}

	for _, phrase := range []string{
		"useful skill",
		"helpful skill",
		"general purpose",
		"for various tasks",
		"for many tasks",
		"this skill helps",
		"helps with tasks",
		"assists with tasks",
		"can be used to",
		"a skill for",
		"tooling support",
	} {
		if strings.Contains(lowerDescription, phrase) {
			return true
		}
	}

	meaningfulWords := make(map[string]struct{})
	for _, word := range strings.FieldsFunc(lowerDescription, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if len(word) < 4 {
			continue
		}

		if _, skip := commonDescriptionWords[word]; skip {
			continue
		}

		meaningfulWords[word] = struct{}{}
	}

	return len(meaningfulWords) < 4
}

func hasNameTitleMismatch(name, title string) bool {
	normalizedName := normalizeComparableText(name)
	normalizedTitle := normalizeComparableText(title)
	if normalizedName == "" || normalizedTitle == "" {
		return false
	}

	if normalizedName == normalizedTitle ||
		strings.Contains(normalizedName, normalizedTitle) ||
		strings.Contains(normalizedTitle, normalizedName) {
		return false
	}

	nameTokens := meaningfulTokens(name)
	titleTokens := meaningfulTokens(title)
	overlap := tokenOverlapCount(nameTokens, titleTokens)

	if overlap == 0 {
		return true
	}

	return overlap == 1 && len(nameTokens) >= 3 && len(titleTokens) >= 3
}

func hasDescriptionBodyMismatch(description string, doc skillDocument) bool {
	descriptionTokens := meaningfulTokens(description)
	bodyTokens := meaningfulTokens(bodyPurposeText(doc))
	if len(descriptionTokens) < 3 || len(bodyTokens) < 6 {
		return false
	}

	overlap := tokenOverlapCount(descriptionTokens, bodyTokens)
	return overlap == 0
}

func findScopeMismatchLine(description string, doc skillDocument) (int, bool) {
	if isBroadScopeText(description) {
		return 0, false
	}

	for _, line := range doc.bodyLines {
		lower := strings.ToLower(line.Trimmed)
		if lower == "" {
			continue
		}

		for _, phrase := range broadScopePhrases {
			if strings.Contains(lower, phrase) {
				return line.Number, true
			}
		}
	}

	return 0, false
}

var commonDescriptionWords = map[string]struct{}{
	"this":      {},
	"that":      {},
	"with":      {},
	"skill":     {},
	"skills":    {},
	"used":      {},
	"using":     {},
	"user":      {},
	"users":     {},
	"helps":     {},
	"help":      {},
	"from":      {},
	"your":      {},
	"into":      {},
	"when":      {},
	"clear":     {},
	"good":      {},
	"more":      {},
	"some":      {},
	"task":      {},
	"tasks":     {},
	"workflow":  {},
	"workflows": {},
	"agent":     {},
	"agents":    {},
	"tool":      {},
	"tools":     {},
	"about":     {},
	"before":    {},
	"after":     {},
	"around":    {},
	"them":      {},
	"they":      {},
	"their":     {},
	"such":      {},
	"than":      {},
	"have":      {},
	"will":      {},
	"would":     {},
	"which":     {},
	"should":    {},
	"only":      {},
	"then":      {},
	"example":   {},
	"examples":  {},
	"usage":     {},
	"input":     {},
	"inputs":    {},
	"output":    {},
	"outputs":   {},
	"request":   {},
	"requests":  {},
	"result":    {},
	"results":   {},
	"response":  {},
	"responses": {},
}

var negativeGuidancePhrases = []string{
	"when not to use",
	"do not use this skill",
	"don't use this skill",
	"avoid using this skill",
	"out of scope",
	"limitations",
	"boundary",
	"boundaries",
	"use another skill",
	"use another tool",
	"not appropriate",
}

var broadScopePhrases = []string{
	"general purpose",
	"for any task",
	"for all tasks",
	"for many tasks",
	"for a wide range of tasks",
	"for many workflows",
	"for almost anything",
	"handles any request",
}

var genericSkillNames = map[string]struct{}{
	"assistant":            {},
	"helper":               {},
	"utility":              {},
	"general helper":       {},
	"general assistant":    {},
	"task helper":          {},
	"workflow helper":      {},
	"multi tool assistant": {},
}

var overbroadTriggerPhrases = []string{
	"any task",
	"almost anything",
	"general purpose",
	"many workflows",
	"many different tasks",
	"whenever you need help",
	"any time you need help",
	"for a wide range of tasks",
}

var genericAssistantPhrases = []string{
	"help with tasks",
	"figure things out",
	"help the user",
	"solve problems",
	"be more helpful",
	"handle requests",
	"general assistance",
}

func genericSkillNameLine(doc skillDocument) (int, bool) {
	if doc.frontMatter.Name != "" && isGenericSkillName(doc.frontMatter.Name) {
		return doc.frontMatter.NameLine, true
	}

	return 0, false
}

func isGenericSkillName(name string) bool {
	normalized := normalizeComparableText(name)
	if normalized == "" {
		return false
	}

	if _, ok := genericSkillNames[normalized]; ok {
		return true
	}

	return false
}

func genericTriggerDescriptionLine(doc skillDocument) (int, bool) {
	if doc.frontMatter.Description == "" {
		return 0, false
	}

	lower := strings.ToLower(doc.frontMatter.Description)
	if isVagueDescription(doc.frontMatter.Description) && lineContainsAny(lower, genericAssistantPhrases) {
		return doc.frontMatter.DescriptionLine, true
	}

	return 0, false
}

func diffuseScopeLine(doc skillDocument) (int, bool) {
	if doc.frontMatter.Description != "" && isDiffuseScopeText(doc.frontMatter.Description) {
		return doc.frontMatter.DescriptionLine, true
	}

	if section, ok := findSection(doc, isWhenToUseHeading); ok {
		if isDiffuseScopeText(joinSectionText(section)) {
			return section.Heading.Line, true
		}
	}

	return 0, false
}

func isDiffuseScopeText(text string) bool {
	lower := strings.ToLower(text)
	if lineContainsAny(lower, broadScopePhrases) {
		return true
	}

	taskCount := 0
	for _, marker := range []string{"debug", "write", "plan", "research", "analyze", "review", "build", "document", "test", "refactor"} {
		if strings.Contains(lower, marker) {
			taskCount++
		}
	}

	return taskCount >= 4
}

func isOverbroadWhenToUseText(text string) bool {
	lower := strings.ToLower(text)
	return lineContainsAny(lower, overbroadTriggerPhrases) || lineContainsAny(lower, genericAssistantPhrases)
}

func lowDistinctivenessLine(doc skillDocument) (int, bool) {
	if doc.frontMatter.Description == "" || sectionText(doc, isWhenToUseHeading) == "" {
		return 0, false
	}

	if line, ok := genericTriggerDescriptionLine(doc); ok && isOverbroadWhenToUseText(sectionText(doc, isWhenToUseHeading)) {
		return line, true
	}

	metadataTokens := triggerConceptTokens(doc.frontMatter.Name + " " + doc.frontMatter.Description)
	whenTokens := triggerConceptTokens(sectionText(doc, isWhenToUseHeading))

	shared := intersectTokens(metadataTokens, whenTokens)
	if len(metadataTokens) >= 3 && len(shared) >= 2 {
		return 0, false
	}

	distinctPool := triggerConceptTokens(doc.frontMatter.Name + " " + doc.frontMatter.Description + " " + sectionText(doc, isWhenToUseHeading))
	if len(distinctPool) <= 3 {
		if doc.frontMatter.DescriptionLine > 0 {
			return doc.frontMatter.DescriptionLine, true
		}
		if doc.hasTitle {
			return doc.title.Line, true
		}
	}

	return 0, false
}

func exampleTriggerMismatchLine(doc skillDocument) (int, bool) {
	if doc.frontMatter.Name == "" || doc.frontMatter.Description == "" {
		return 0, false
	}

	exampleSection, ok := findSection(doc, func(heading heading) bool {
		return heading.Level > 1 && strings.Contains(normalizeHeading(heading.Text), "example")
	})
	if !ok {
		return 0, false
	}

	metadataTokens := triggerConceptTokens(doc.frontMatter.Name + " " + doc.frontMatter.Description + " " + sectionText(doc, isWhenToUseHeading))
	exampleTokens := triggerConceptTokens(joinSectionText(exampleSection))
	if len(metadataTokens) < 3 || len(exampleTokens) < 3 {
		return 0, false
	}

	if len(intersectTokens(metadataTokens, exampleTokens)) == 0 && (isGenericExamplesText(joinSectionText(exampleSection)) || countExampleSignalLines(exampleSection) <= 1) {
		return exampleSection.Heading.Line, true
	}

	return 0, false
}

func weakTriggerPatternLine(doc skillDocument) (int, bool) {
	exampleSection, ok := findSection(doc, func(heading heading) bool {
		return heading.Level > 1 && strings.Contains(normalizeHeading(heading.Text), "example")
	})
	if !ok {
		return 0, false
	}

	linesWithSignals := countExampleSignalLines(exampleSection)

	if linesWithSignals == 0 {
		return exampleSection.Heading.Line, true
	}

	if linesWithSignals >= 2 && !isGenericExamplesText(joinSectionText(exampleSection)) {
		return 0, false
	}

	metadataTokens := triggerConceptTokens(doc.frontMatter.Name + " " + doc.frontMatter.Description + " " + sectionText(doc, isWhenToUseHeading))
	exampleTokens := triggerConceptTokens(joinSectionText(exampleSection))
	if len(metadataTokens) >= 3 && len(intersectTokens(metadataTokens, exampleTokens)) <= 1 && linesWithSignals <= 2 {
		return exampleSection.Heading.Line, true
	}

	return 0, false
}

func countExampleSignalLines(section section) int {
	linesWithSignals := 0
	for _, line := range section.Lines {
		lower := strings.ToLower(line.Trimmed)
		if strings.Contains(lower, "request:") || strings.Contains(lower, "invocation:") || strings.Contains(lower, "input:") || strings.Contains(lower, "prompt:") || lineContainsInstructionSignal(lower) {
			linesWithSignals++
		}
	}
	return linesWithSignals
}

func triggerScopeInconsistencyLine(doc skillDocument) (int, bool) {
	whenText := sectionText(doc, isWhenToUseHeading)
	if whenText == "" {
		return 0, false
	}

	titleTokens := triggerConceptTokens(doc.title.Text)
	descriptionTokens := triggerConceptTokens(doc.frontMatter.Description)
	whenTokens := triggerConceptTokens(whenText)

	if len(whenTokens) < 3 {
		return 0, false
	}

	if len(titleTokens) >= 2 && len(intersectTokens(titleTokens, whenTokens)) == 0 {
		return whenToUseLine(doc), true
	}

	if len(descriptionTokens) >= 2 && len(intersectTokens(descriptionTokens, whenTokens)) == 0 {
		return whenToUseLine(doc), true
	}

	return 0, false
}

func triggerConceptTokens(text string) []string {
	filtered := make([]string, 0)
	for _, token := range meaningfulTokens(text) {
		if _, skip := genericTriggerWords[token]; skip {
			continue
		}
		filtered = append(filtered, token)
	}
	return filtered
}

var genericTriggerWords = map[string]struct{}{
	"validate":   {},
	"local":      {},
	"before":     {},
	"publish":    {},
	"publishing": {},
	"changes":    {},
	"reusable":   {},
	"clear":      {},
	"practical":  {},
}

func intersectTokens(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	rightSet := make(map[string]struct{}, len(right))
	for _, token := range right {
		rightSet[canonicalToken(token)] = struct{}{}
	}

	shared := make([]string, 0)
	seen := make(map[string]struct{})
	for _, token := range left {
		canonical := canonicalToken(token)
		if _, ok := rightSet[canonical]; !ok {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		shared = append(shared, token)
	}

	return shared
}

func sectionText(doc skillDocument, match func(heading heading) bool) string {
	section, ok := findSection(doc, match)
	if !ok {
		return ""
	}

	return joinSectionText(section)
}

func whenToUseLine(doc skillDocument) int {
	section, ok := findSection(doc, isWhenToUseHeading)
	if !ok {
		return 0
	}

	return section.Heading.Line
}

func normalizeHeading(text string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	return strings.Join(fields, " ")
}

func normalizeComparableText(text string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(strings.TrimSpace(text)), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}), " ")
}

func meaningfulTokens(text string) []string {
	seen := make(map[string]struct{})
	tokens := make([]string, 0)

	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if len(token) < 4 {
			continue
		}

		if _, skip := commonDescriptionWords[token]; skip {
			continue
		}

		if _, exists := seen[token]; exists {
			continue
		}

		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}

	return tokens
}

func tokenOverlapCount(left, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}

	rightSet := make(map[string]struct{}, len(right))
	for _, token := range right {
		rightSet[canonicalToken(token)] = struct{}{}
	}

	count := 0
	for _, token := range left {
		if _, ok := rightSet[canonicalToken(token)]; ok {
			count++
		}
	}

	return count
}

func canonicalToken(token string) string {
	if len(token) <= 6 {
		return token
	}

	return token[:6]
}

func bodyPurposeText(doc skillDocument) string {
	parts := make([]string, 0, 10)
	if doc.hasTitle {
		parts = append(parts, doc.title.Text)
	}

	for _, section := range doc.sections {
		if section.Heading.Level > 2 {
			continue
		}

		normalized := normalizeHeading(section.Heading.Text)
		if isNegativeGuidanceHeading(section.Heading) || strings.Contains(normalized, "example") {
			continue
		}

		parts = append(parts, section.Heading.Text)
		for _, line := range section.Lines {
			if line.Trimmed == "" {
				continue
			}

			parts = append(parts, line.Trimmed)
			if len(parts) >= 10 {
				return strings.Join(parts, " ")
			}
		}
	}

	for _, line := range doc.bodyLines {
		if line.Trimmed == "" || strings.HasPrefix(line.Trimmed, "#") {
			continue
		}

		parts = append(parts, line.Trimmed)
		if len(parts) >= 10 {
			break
		}
	}

	return strings.Join(parts, " ")
}

func findSection(doc skillDocument, match func(heading heading) bool) (section, bool) {
	for _, section := range doc.sections {
		if match(section.Heading) {
			return section, true
		}
	}

	return section{}, false
}

func joinSectionText(section section) string {
	parts := make([]string, 0, len(section.Lines))
	for _, line := range section.Lines {
		if line.Trimmed == "" {
			continue
		}

		parts = append(parts, line.Trimmed)
	}

	return strings.Join(parts, "\n")
}

func lowerJoinedBodyText(lines []documentLine) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		if line.Trimmed == "" {
			continue
		}

		parts = append(parts, strings.ToLower(line.Trimmed))
	}

	return strings.Join(parts, "\n")
}

func isNegativeGuidanceHeading(heading heading) bool {
	normalized := normalizeHeading(heading.Text)
	return strings.Contains(normalized, "when not to use") ||
		strings.Contains(normalized, "limitations") ||
		strings.Contains(normalized, "out of scope") ||
		strings.Contains(normalized, "boundary") ||
		strings.Contains(normalized, "boundaries")
}

func findBodyLine(doc skillDocument, phrases []string) (int, string, bool) {
	for _, line := range doc.bodyLines {
		lower := strings.ToLower(line.Trimmed)
		if lower == "" {
			continue
		}

		for _, phrase := range phrases {
			if strings.Contains(lower, phrase) {
				return line.Number, line.Trimmed, true
			}
		}
	}

	return 0, "", false
}

func isWeakNegativeGuidanceText(text string, strictness lint.Strictness) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return true
	}

	if len([]rune(lower)) < weakNegativeGuidanceThresholdFor(strictness) {
		return true
	}

	for _, phrase := range []string{
		"use judgment",
		"depends on context",
		"not for everything",
		"may not always fit",
		"consider alternatives",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	for _, phrase := range []string{"instead", "avoid", "unless", "only if", "out of scope", "limitation", "do not use"} {
		if strings.Contains(lower, phrase) {
			return false
		}
	}

	return true
}

func isWeakExamplesSection(section section, strictness lint.Strictness) bool {
	text := joinSectionText(section)
	if len([]rune(strings.TrimSpace(text))) < weakExamplesRuneThresholdFor(strictness) {
		return true
	}

	nonEmptyLines := 0
	for _, line := range section.Lines {
		if line.Trimmed != "" {
			nonEmptyLines++
		}
	}

	return nonEmptyLines < 2
}

func isGenericExamplesText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}

	genericPhrases := []string{
		"example scenario",
		"for example",
		"example request",
		"use this skill",
		"run the command",
		"try this skill",
	}
	for _, phrase := range genericPhrases {
		if strings.Contains(lower, phrase) && !hasConcreteExampleSignal(lower) {
			return true
		}
	}

	return false
}

func hasExampleTriggerInput(section section) bool {
	for _, line := range section.Lines {
		lower := strings.ToLower(line.Trimmed)
		if strings.Contains(lower, "request:") ||
			strings.Contains(lower, "input:") ||
			strings.Contains(lower, "prompt:") ||
			strings.Contains(lower, "user:") ||
			strings.Contains(lower, "invocation:") ||
			strings.Contains(lower, "run `") ||
			strings.Contains(lower, "ask ") {
			return true
		}
	}

	return false
}

func hasExampleExpectedOutcome(section section) bool {
	for _, line := range section.Lines {
		if isExpectedOutcomeLine(line.Trimmed) {
			return true
		}
	}

	return false
}

func isExpectedOutcomeLine(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "result:") ||
		strings.Contains(lower, "output:") ||
		strings.Contains(lower, "expected:") ||
		strings.Contains(lower, "returns") ||
		strings.Contains(lower, "produces") ||
		strings.Contains(lower, "response:") ||
		strings.Contains(lower, "you should see")
}

func abstractExampleLine(section section) int {
	for _, line := range section.Lines {
		lower := strings.ToLower(stripExamplePrefix(line.Trimmed))
		if lower == "" || hasConcreteExampleSignal(lower) {
			continue
		}

		for _, phrase := range []string{
			"help me with something",
			"do the thing",
			"handle the request",
			"perform the task",
			"use this skill when needed",
			"work on the task",
			"fix the issue",
			"review the thing",
			"general help",
			"something relevant",
		} {
			if strings.Contains(lower, phrase) {
				return line.Number
			}
		}
	}

	return 0
}

func placeholderHeavyExampleLine(section section) int {
	placeholderCount := 0
	firstLine := 0

	for _, line := range section.Lines {
		count := countPlaceholderMarkers(strings.ToLower(line.Trimmed))
		if count == 0 {
			continue
		}

		placeholderCount += count
		if firstLine == 0 {
			firstLine = line.Number
		}
	}

	if placeholderCount >= placeholderExampleThreshold {
		return firstLine
	}

	return 0
}

func countPlaceholderMarkers(text string) int {
	count := 0
	for _, marker := range []string{
		"<path>",
		"<file>",
		"<skill>",
		"<name>",
		"<request>",
		"<input>",
		"<output>",
		"{{",
		"}}",
		"your-file",
		"your request",
		"path/to/",
		"todo",
		"tbd",
	} {
		count += strings.Count(text, marker)
	}

	return count
}

func missingExampleOutcomeLine(section section, inspection exampleInspection) int {
	if !inspection.HasTriggerInput || inspection.HasExpectedOutcome {
		return 0
	}

	if len(section.Lines) < 4 {
		return 0
	}

	for _, line := range section.Lines {
		lower := strings.ToLower(line.Trimmed)
		if strings.Contains(lower, "request:") ||
			strings.Contains(lower, "input:") ||
			strings.Contains(lower, "prompt:") ||
			strings.Contains(lower, "invocation:") {
			return line.Number
		}
	}

	return section.Heading.Line
}

func missingExampleTriggerLine(section section, inspection exampleInspection) int {
	if inspection.HasTriggerInput || !inspection.HasExpectedOutcome {
		return 0
	}

	for _, line := range section.Lines {
		if isExpectedOutcomeLine(line.Trimmed) {
			return line.Number
		}
	}

	return section.Heading.Line
}

func exampleScopeContradictionLine(doc skillDocument, section section) int {
	negativeText := sectionText(doc, isNegativeGuidanceHeading)
	if negativeText == "" {
		return 0
	}

	forbidden := meaningfulTokens(negativeText)
	if len(forbidden) < 2 {
		return 0
	}

	for _, line := range section.Lines {
		lower := strings.ToLower(line.Trimmed)
		if strings.Contains(lower, "do not use") || strings.Contains(lower, "avoid") {
			continue
		}

		if tokenOverlapCount(forbidden, meaningfulTokens(line.Trimmed)) >= 2 {
			return line.Number
		}
	}

	return 0
}

func exampleGuidanceMismatchLine(doc skillDocument, section section, inspection exampleInspection) int {
	whenText := sectionText(doc, isWhenToUseHeading)
	if whenText == "" || inspection.Generic || !inspection.HasTriggerInput {
		return 0
	}

	whenTokens := triggerConceptTokens(whenText)
	exampleTokens := triggerConceptTokens(joinSectionText(section))
	if len(whenTokens) < 3 || len(exampleTokens) < 3 {
		return 0
	}

	if len(intersectTokens(whenTokens, exampleTokens)) > 0 {
		return 0
	}

	for _, line := range section.Lines {
		lower := strings.ToLower(line.Trimmed)
		if strings.Contains(lower, "request:") ||
			strings.Contains(lower, "input:") ||
			strings.Contains(lower, "prompt:") ||
			strings.Contains(lower, "invocation:") {
			return line.Number
		}
	}

	return section.Heading.Line
}

func incompleteExampleLine(section section) int {
	for _, line := range section.Lines {
		lower := strings.ToLower(strings.TrimSpace(line.Trimmed))
		if lower == "" {
			continue
		}

		if strings.Contains(lower, "todo") ||
			strings.Contains(lower, "tbd") ||
			strings.Contains(lower, "[fill in") ||
			strings.Contains(lower, "coming soon") ||
			strings.HasSuffix(lower, "...") {
			return line.Number
		}
	}

	return 0
}

func missingExampleBundleResource(doc skillDocument, section section) (string, int) {
	for _, line := range section.Lines {
		for _, candidate := range exampleResourcePaths(line.Text) {
			normalizedPath := filepath.ToSlash(cleanLinkPath(candidate))
			if normalizedPath == "" || normalizedPath == "." {
				continue
			}

			resolvedPath := resolveLinkPath(doc.skillDir, normalizedPath)
			if !isWithinSkillRoot(doc.skillDir, resolvedPath) {
				continue
			}

			if _, ok := lookupBundleEntry(doc.bundle, doc.skillDir, normalizedPath); !ok {
				return normalizedPath, line.Number
			}
		}
	}

	return "", 0
}

func exampleResourcePaths(text string) []string {
	candidates := make([]string, 0)
	seen := make(map[string]struct{})

	add := func(path string) {
		path = strings.TrimSpace(strings.Trim(path, ".,:;()[]\"'"))
		if path == "" || !looksLikeLocalResourceMention(path) {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	for _, match := range codeSpanPattern.FindAllStringSubmatch(text, -1) {
		if len(match) >= 2 {
			add(match[1])
		}
	}

	withoutLinks := markdownLinkPattern.ReplaceAllString(text, " ")
	for _, match := range plainPathPattern.FindAllStringSubmatch(withoutLinks, -1) {
		if len(match) >= 2 {
			add(match[1])
		}
	}

	return candidates
}

func lowVarietyExampleLine(section section) int {
	lines := exampleScenarioLines(section)
	if len(lines) < 3 {
		return 0
	}

	similarPairs := 0
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			if exampleLinesAreSimilar(lines[i].Trimmed, lines[j].Trimmed) {
				similarPairs++
			}
		}
	}

	if similarPairs >= len(lines)-1 {
		return lines[1].Number
	}

	return 0
}

func exampleScenarioLines(section section) []documentLine {
	lines := make([]documentLine, 0)
	for _, line := range section.Lines {
		lower := strings.ToLower(line.Trimmed)
		if strings.Contains(lower, "request:") ||
			strings.Contains(lower, "input:") ||
			strings.Contains(lower, "prompt:") ||
			strings.Contains(lower, "example:") {
			lines = append(lines, line)
		}
	}

	if len(lines) >= 2 {
		return lines
	}

	return section.Lines
}

func exampleLinesAreSimilar(left, right string) bool {
	leftTokens := exampleSimilarityTokens(left)
	rightTokens := exampleSimilarityTokens(right)
	if len(leftTokens) < 4 || len(rightTokens) < 4 {
		return false
	}

	shared := tokenOverlapCount(leftTokens, rightTokens)
	shorter := len(leftTokens)
	if len(rightTokens) < shorter {
		shorter = len(rightTokens)
	}

	return shared*100 >= shorter*60
}

func exampleSimilarityTokens(text string) []string {
	lower := stripExamplePrefix(strings.ToLower(text))
	lower = strings.ReplaceAll(lower, "local", "")
	lower = strings.ReplaceAll(lower, "skill", "")
	lower = strings.ReplaceAll(lower, "directory", "")
	lower = strings.ReplaceAll(lower, "bundle", "")
	return meaningfulTokens(lower)
}

func stripExamplePrefix(text string) string {
	trimmed := strings.TrimSpace(text)
	for _, prefix := range []string{
		"- request:",
		"- invocation:",
		"- result:",
		"- input:",
		"- output:",
		"- example:",
		"request:",
		"invocation:",
		"result:",
		"input:",
		"output:",
		"example:",
		"prompt:",
		"user:",
	} {
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}

	return trimmed
}

func hasConcreteExampleSignal(text string) bool {
	for _, phrase := range []string{
		"firety skill lint",
		"input:",
		"output:",
		"request:",
		"prompt:",
		"result:",
		"returns",
		"produces",
		"docs/",
		"skill root",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}

	return false
}

func hasExampleInvocationPattern(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range []string{
		"invocation:",
		"run `",
		"ask `",
		"invoke `",
		"prompt:",
		"input:",
		"user:",
		"firety skill lint",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	return false
}

func isBroadScopeText(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range broadScopePhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	return false
}

func analyzePortability(doc skillDocument) portabilityAssessment {
	assessment := portabilityAssessment{
		profileScores: make(map[SkillLintProfile]int),
	}

	assessment.declaredTarget, assessment.declaredTargetLine = declaredTargetProfile(doc)
	assessment.declaredGeneric, assessment.declaredGenericLine = declaredGenericPortability(doc)
	assessment.hasBoundaryGuidance, assessment.boundaryLine = hasToolTargetBoundaryGuidance(doc, assessment.declaredTarget)
	assessment.mixedLine = mixedEcosystemGuidanceLine(doc)

	highestScore := 0
	secondScore := 0
	for _, line := range portabilityLines(doc) {
		if line.Trimmed == "" {
			continue
		}

		signals := collectPortabilitySignals(line.Trimmed)
		if len(signals) == 0 {
			continue
		}

		for _, signal := range signals {
			score := portabilitySignalWeight(signal)
			assessment.profileScores[signal.profile] += score

			if assessment.profileScores[signal.profile] > highestScore {
				secondScore = highestScore
				highestScore = assessment.profileScores[signal.profile]
				assessment.dominantProfile = signal.profile
				assessment.dominantProfileLine = line.Number
				continue
			}

			if assessment.profileScores[signal.profile] > secondScore {
				secondScore = assessment.profileScores[signal.profile]
			}
		}
	}

	if highestScore < 3 || highestScore-secondScore < 2 {
		assessment.dominantProfile = ""
		assessment.dominantProfileLine = 0
	}

	if line, profile, ok := exampleEcosystemMismatch(doc, assessment); ok {
		assessment.exampleMismatchLine = line
		assessment.exampleMismatchProfile = profile
	}

	return assessment
}

func declaredTargetProfile(doc skillDocument) (SkillLintProfile, int) {
	candidates := []documentLine{}
	if doc.frontMatter.Name != "" {
		candidates = append(candidates, documentLine{Number: doc.frontMatter.NameLine, Trimmed: doc.frontMatter.Name})
	}
	if doc.hasTitle {
		candidates = append(candidates, documentLine{Number: doc.title.Line, Trimmed: doc.title.Text})
	}
	if doc.frontMatter.Description != "" {
		candidates = append(candidates, documentLine{Number: doc.frontMatter.DescriptionLine, Trimmed: doc.frontMatter.Description})
	}
	if section, ok := findSection(doc, isWhenToUseHeading); ok {
		candidates = append(candidates, documentLine{Number: section.Heading.Line, Trimmed: joinSectionText(section)})
	}

	for _, line := range candidates {
		profiles := namedTargetProfiles(line.Trimmed)
		if len(profiles) == 1 {
			return profiles[0], line.Number
		}
	}

	return "", 0
}

func namedTargetProfiles(text string) []SkillLintProfile {
	lower := strings.ToLower(text)
	profiles := make([]SkillLintProfile, 0, 2)
	for _, candidate := range portabilityProfiles {
		if !lineContainsAny(lower, candidate.brandTerms) {
			continue
		}

		if strings.Contains(lower, "for "+strings.ToLower(candidate.displayName)) ||
			strings.Contains(lower, strings.ToLower(candidate.displayName)+" skill") ||
			strings.Contains(lower, strings.ToLower(candidate.displayName)+" users") ||
			strings.Contains(lower, "use "+strings.ToLower(candidate.displayName)) ||
			strings.Contains(lower, "inside "+strings.ToLower(candidate.displayName)) ||
			strings.Contains(lower, "within "+strings.ToLower(candidate.displayName)) ||
			strings.Contains(lower, "targeted at "+strings.ToLower(candidate.displayName)) ||
			strings.Contains(lower, strings.ToLower(candidate.displayName)+" workflow") ||
			strings.EqualFold(strings.TrimSpace(text), candidate.displayName) {
			profiles = append(profiles, candidate.profile)
		}
	}

	return profiles
}

func declaredGenericPortability(doc skillDocument) (bool, int) {
	lines := []documentLine{}
	if doc.frontMatter.Description != "" {
		lines = append(lines, documentLine{Number: doc.frontMatter.DescriptionLine, Trimmed: doc.frontMatter.Description})
	}
	if section, ok := findSection(doc, isWhenToUseHeading); ok {
		lines = append(lines, documentLine{Number: section.Heading.Line, Trimmed: joinSectionText(section)})
	}
	if section, ok := findSection(doc, isNegativeGuidanceHeading); ok {
		lines = append(lines, documentLine{Number: section.Heading.Line, Trimmed: joinSectionText(section)})
	}

	for _, line := range lines {
		lower := strings.ToLower(line.Trimmed)
		for _, phrase := range []string{
			"portable",
			"cross-tool",
			"cross tool",
			"tool-agnostic",
			"generic skill",
			"generic profile",
			"across tools",
			"multiple tools",
			"works across",
		} {
			if strings.Contains(lower, phrase) {
				return true, line.Number
			}
		}
	}

	return false, 0
}

func mixedEcosystemGuidanceLine(doc skillDocument) int {
	for _, match := range []func(heading heading) bool{
		func(heading heading) bool {
			normalized := normalizeHeading(heading.Text)
			return strings.Contains(normalized, "usage") ||
				strings.Contains(normalized, "invoke") ||
				strings.Contains(normalized, "invocation")
		},
		func(heading heading) bool {
			return heading.Level > 1 && strings.Contains(normalizeHeading(heading.Text), "example")
		},
	} {
		section, ok := findSection(doc, match)
		if !ok {
			continue
		}

		for _, line := range section.Lines {
			if lineContainsMixedProfiles(collectPortabilitySignals(line.Trimmed)) {
				return line.Number
			}
		}
	}

	return 0
}

func hasToolTargetBoundaryGuidance(doc skillDocument, target SkillLintProfile) (bool, int) {
	sections := []section{}
	if section, ok := findSection(doc, isNegativeGuidanceHeading); ok {
		sections = append(sections, section)
	}
	if section, ok := findSection(doc, isWhenToUseHeading); ok {
		sections = append(sections, section)
	}

	targetName := strings.ToLower(profileDisplayName(target))
	for _, section := range sections {
		text := strings.ToLower(joinSectionText(section))
		if text == "" {
			continue
		}

		if target != "" && (strings.Contains(text, targetName+" users") ||
			strings.Contains(text, "only for "+targetName) ||
			strings.Contains(text, "outside "+targetName) ||
			strings.Contains(text, "not for "+targetName)) {
			return true, section.Heading.Line
		}

		for _, phrase := range []string{
			"tool-specific",
			"not portable",
			"outside this tool",
			"outside this workflow",
			"use another tool",
			"do not use this skill outside",
			"only use this skill in",
		} {
			if strings.Contains(text, phrase) {
				return true, section.Heading.Line
			}
		}

		if target != "" {
			for _, candidate := range portabilityProfiles {
				if candidate.profile == target {
					continue
				}
				if lineContainsAny(text, candidate.brandTerms) {
					return true, section.Heading.Line
				}
			}
		}
	}

	return false, 0
}

func portabilitySignalWeight(signal portabilitySignal) int {
	weight := 0
	if signal.hasBranding {
		weight++
	}
	if signal.hasInstallPath {
		weight += 2
	}
	if signal.hasInvocationTerm {
		weight += 2
	}
	return weight
}

func portabilityPrimaryLine(assessment portabilityAssessment) int {
	if assessment.declaredGenericLine > 0 {
		return assessment.declaredGenericLine
	}
	if assessment.declaredTargetLine > 0 {
		return assessment.declaredTargetLine
	}
	if assessment.dominantProfileLine > 0 {
		return assessment.dominantProfileLine
	}
	return 0
}

func portabilityExpectedProfile(assessment portabilityAssessment) SkillLintProfile {
	if assessment.declaredTarget != "" {
		return assessment.declaredTarget
	}
	return assessment.dominantProfile
}

func profileStronglyDisagrees(profile SkillLintProfile, assessment portabilityAssessment) bool {
	expected := portabilityExpectedProfile(assessment)
	return expected != "" && expected != profile
}

func exampleEcosystemMismatch(doc skillDocument, assessment portabilityAssessment) (int, SkillLintProfile, bool) {
	expected := portabilityExpectedProfile(assessment)
	if expected == "" {
		return 0, "", false
	}

	section, ok := findSection(doc, func(heading heading) bool {
		return heading.Level > 1 && strings.Contains(normalizeHeading(heading.Text), "example")
	})
	if !ok {
		return 0, "", false
	}

	for _, line := range section.Lines {
		signals := collectPortabilitySignals(line.Trimmed)
		for _, signal := range signals {
			if signal.profile != expected {
				return line.Number, signal.profile, true
			}
		}
	}

	return 0, "", false
}

func (assessment portabilityAssessment) isClearlyTargeted() bool {
	return assessment.declaredTarget != "" &&
		(assessment.dominantProfile == "" || assessment.dominantProfile == assessment.declaredTarget) &&
		assessment.mixedLine == 0 &&
		!assessment.declaredGeneric
}

func (assessment portabilityAssessment) isClearlyTargetedAt(profile SkillLintProfile) bool {
	return assessment.isClearlyTargeted() && assessment.declaredTarget == profile
}

func (assessment portabilityAssessment) isHonestTargetedLine(signals []portabilitySignal) bool {
	if !assessment.isClearlyTargeted() {
		return false
	}

	for _, signal := range signals {
		if signal.profile == assessment.declaredTarget {
			return true
		}
	}

	return false
}

func portabilityLines(doc skillDocument) []documentLine {
	lines := make([]documentLine, 0, len(doc.bodyLines)+2)

	if doc.frontMatter.HasName && doc.frontMatter.Name != "" {
		lines = append(lines, documentLine{
			Number:  doc.frontMatter.NameLine,
			Text:    doc.frontMatter.Name,
			Trimmed: doc.frontMatter.Name,
		})
	}

	if doc.frontMatter.HasDescription && doc.frontMatter.Description != "" {
		lines = append(lines, documentLine{
			Number:  doc.frontMatter.DescriptionLine,
			Text:    doc.frontMatter.Description,
			Trimmed: doc.frontMatter.Description,
		})
	}

	lines = append(lines, doc.bodyLines...)
	return lines
}

func collectPortabilitySignals(text string) []portabilitySignal {
	lower := strings.ToLower(text)
	signals := make([]portabilitySignal, 0, len(portabilityProfiles))

	for _, candidate := range portabilityProfiles {
		signal := portabilitySignal{profile: candidate.profile}

		signal.hasBranding = lineContainsAny(lower, candidate.brandTerms)
		signal.hasInstallPath = lineContainsAny(lower, candidate.installMarkers)
		signal.hasInvocationTerm = lineContainsAny(lower, candidate.invocationMarkers) ||
			(lineContainsAny(lower, candidate.instructionMarkers) && lineContainsInstructionSignal(lower))

		if !signal.hasBranding && !signal.hasInstallPath && !signal.hasInvocationTerm {
			continue
		}

		signals = append(signals, signal)
	}

	return signals
}

func lineContainsMixedProfiles(signals []portabilitySignal) bool {
	if len(signals) < 2 {
		return false
	}

	seen := make(map[SkillLintProfile]struct{}, len(signals))
	for _, signal := range signals {
		seen[signal.profile] = struct{}{}
	}

	return len(seen) > 1
}

func hasStrongBrandingLock(signal portabilitySignal, signals []portabilitySignal) bool {
	if signal.hasInstallPath || signal.hasInvocationTerm {
		return true
	}

	brandCount := 0
	for _, current := range signals {
		if current.hasBranding {
			brandCount++
		}
	}

	return signal.hasBranding && brandCount == 1
}

func lineContainsInstructionSignal(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range []string{
		"run ",
		"use ",
		"invoke ",
		"install ",
		"open ",
		"press ",
		"put ",
		"place ",
		"save ",
		"store ",
		"request:",
		"invocation:",
		"usage",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	return false
}

func lineContainsAny(text string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}

	return false
}

func profileDisplayName(profile SkillLintProfile) string {
	for _, candidate := range portabilityProfiles {
		if candidate.profile == profile {
			return candidate.displayName
		}
	}

	if profile == SkillLintProfileGeneric {
		return "generic"
	}

	return string(profile)
}

func looksLikeLocalResourceMention(candidate string) bool {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\n") {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
		return false
	}

	if strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "~") {
		return false
	}

	if strings.HasSuffix(trimmed, "/") {
		return true
	}

	if strings.Contains(trimmed, "/") {
		return true
	}

	extension := strings.ToLower(filepath.Ext(trimmed))
	return extension != "" && extension != "."
}

func lookupBundleEntry(bundle bundleInventory, skillDir, targetPath string) (bundleEntry, bool) {
	resolvedPath := resolveLinkPath(skillDir, targetPath)
	relativePath, err := filepath.Rel(skillDir, resolvedPath)
	if err != nil {
		return bundleEntry{}, false
	}

	relativePath = filepath.ToSlash(filepath.Clean(relativePath))
	entry, ok := bundle.files[relativePath]
	return entry, ok
}

func isWithinSkillRoot(skillDir, targetPath string) bool {
	relativePath, err := filepath.Rel(skillDir, targetPath)
	if err != nil {
		return false
	}

	relativePath = filepath.ToSlash(filepath.Clean(relativePath))
	return relativePath != ".." && !strings.HasPrefix(relativePath, "../")
}

func isSuspiciousReferencedResource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".dll", ".dylib", ".so", ".bin", ".pkg", ".app", ".jar":
		return true
	default:
		return false
	}
}

func isEmptyReferencedResource(path string, size int64) bool {
	if size > 0 {
		return false
	}

	normalized := filepath.ToSlash(strings.ToLower(path))
	extension := strings.ToLower(filepath.Ext(normalized))
	return isScriptLikeResource(extension) ||
		isTextLikeResource(extension) ||
		strings.Contains(normalized, "/examples/") ||
		strings.Contains(normalized, "/scripts/") ||
		strings.HasPrefix(normalized, "examples/") ||
		strings.HasPrefix(normalized, "scripts/")
}

func isUnhelpfulReferencedResource(path string, size int64) bool {
	if size == 0 || size >= unhelpfulResourceByteThreshold {
		return false
	}

	return isTextLikeResource(strings.ToLower(filepath.Ext(path)))
}

func isScriptLikeResource(extension string) bool {
	switch extension {
	case ".sh", ".bash", ".zsh", ".py", ".js", ".ts", ".rb", ".go", ".ps1":
		return true
	default:
		return false
	}
}

func isTextLikeResource(extension string) bool {
	switch extension {
	case ".md", ".txt", ".json", ".yaml", ".yml", ".toml":
		return true
	default:
		return false
	}
}

func referencedTopLevelDirs(doc skillDocument) map[string]struct{} {
	referenced := make(map[string]struct{})

	addPath := func(raw string) {
		normalized := filepath.ToSlash(cleanLinkPath(raw))
		if normalized == "." || normalized == "" {
			return
		}

		topLevel := normalized
		if slashIndex := strings.Index(topLevel, "/"); slashIndex >= 0 {
			topLevel = topLevel[:slashIndex]
		}

		referenced[topLevel] = struct{}{}
	}

	for _, link := range doc.links {
		if isLocalLink(link.Destination) {
			addPath(link.Destination)
		}
	}

	for _, mention := range doc.mentions {
		addPath(mention.Path)
	}

	return referenced
}

func hasStrongBundleExpectation(doc skillDocument) bool {
	referenced := referencedTopLevelDirs(doc)
	for _, helperDir := range []string{"scripts", "examples", "assets"} {
		if _, ok := referenced[helperDir]; ok {
			return true
		}
	}

	return len(doc.links)+len(doc.mentions) >= 2
}

func estimateTokenCount(text string) int {
	runes := len([]rune(text))
	if runes == 0 {
		return 0
	}

	return (runes + 3) / 4
}

func estimateTokenCountFromBytes(size int64) int {
	if size <= 0 {
		return 0
	}

	return int((size + 3) / 4)
}

func estimateInstructionTokens(doc skillDocument) int {
	total := 0
	for _, section := range doc.sections {
		normalizedHeading := normalizeHeading(section.Heading.Text)
		if section.Heading.Level <= 1 || strings.Contains(normalizedHeading, "example") {
			continue
		}

		total += estimateTokenCount(joinSectionText(section))
	}

	return total
}

func hasDuplicateExamples(section section) bool {
	seen := make(map[string]int)

	for _, line := range section.Lines {
		canonical := canonicalExampleLine(line.Trimmed)
		if canonical == "" {
			continue
		}

		seen[canonical]++
		if seen[canonical] >= 2 {
			return true
		}
	}

	return false
}

func canonicalExampleLine(line string) string {
	normalized := strings.ToLower(strings.TrimSpace(line))
	if normalized == "" {
		return ""
	}

	for _, prefix := range []string{
		"- request:",
		"- invocation:",
		"- result:",
		"request:",
		"invocation:",
		"result:",
		"output:",
		"input:",
		"example:",
	} {
		normalized = strings.TrimPrefix(normalized, prefix)
	}

	normalized = strings.Join(strings.FieldsFunc(normalized, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}), " ")

	if len(normalized) < 20 {
		return ""
	}

	return normalized
}

func isRepetitiveInstructionSection(section section) bool {
	seen := make(map[string]int)

	for _, line := range section.Lines {
		canonical := canonicalInstructionLine(line.Trimmed)
		if canonical == "" {
			continue
		}

		seen[canonical]++
		if seen[canonical] >= 3 {
			return true
		}
	}

	return false
}

func canonicalInstructionLine(line string) string {
	normalized := strings.ToLower(strings.TrimSpace(line))
	if normalized == "" {
		return ""
	}

	normalized = strings.Join(strings.FieldsFunc(normalized, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}), " ")
	if len(normalized) < 24 {
		return ""
	}

	return normalized
}

func isLocalLink(destination string) bool {
	if destination == "" || strings.HasPrefix(destination, "#") {
		return false
	}

	parsed, err := url.Parse(destination)
	if err == nil && (parsed.Scheme != "" || strings.HasPrefix(destination, "//")) {
		return false
	}

	return true
}

func cleanLinkPath(destination string) string {
	pathOnly := destination

	if fragmentIndex := strings.Index(pathOnly, "#"); fragmentIndex >= 0 {
		pathOnly = pathOnly[:fragmentIndex]
	}

	if queryIndex := strings.Index(pathOnly, "?"); queryIndex >= 0 {
		pathOnly = pathOnly[:queryIndex]
	}

	return filepath.Clean(filepath.FromSlash(pathOnly))
}

func resolveLinkPath(skillDir, targetPath string) string {
	if filepath.IsAbs(targetPath) {
		return targetPath
	}

	return filepath.Join(skillDir, targetPath)
}

func yamlNodeLine(rawStartLine int, relativeLine int) int {
	if relativeLine <= 0 {
		return rawStartLine
	}

	return rawStartLine + relativeLine - 1
}

func yamlErrorLineFromMessage(rawStartLine int, err error) int {
	matches := yamlErrorLinePattern.FindStringSubmatch(err.Error())
	if len(matches) < 2 {
		return rawStartLine
	}

	line, parseErr := strconv.Atoi(matches[1])
	if parseErr != nil {
		return rawStartLine
	}

	return yamlNodeLine(rawStartLine, line)
}
