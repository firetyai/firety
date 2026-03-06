package service

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	workspacepkg "github.com/firety/firety/internal/domain/workspace"
)

var (
	workspaceMarkdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	workspaceCodePathPattern     = regexp.MustCompile("`([^`]+/[^`]+)`")
)

type WorkspaceChangeOptions struct {
	BaseRev string
	HeadRev string
}

func (o WorkspaceChangeOptions) Validate() error {
	if strings.TrimSpace(o.HeadRev) != "" && strings.TrimSpace(o.BaseRev) == "" {
		return fmt.Errorf("--head requires --base")
	}
	return nil
}

func (s WorkspaceService) Changes(root string, options WorkspaceChangeOptions) (workspacepkg.ChangeScope, error) {
	if err := options.Validate(); err != nil {
		return workspacepkg.ChangeScope{}, err
	}

	discovery, err := discoverWorkspace(root)
	if err != nil {
		return workspacepkg.ChangeScope{}, err
	}

	changedFiles, diffContext, err := workspaceChangedFiles(discovery.WorkspaceRoot, options)
	if err != nil {
		return workspacepkg.ChangeScope{}, err
	}

	directByPath := make(map[string]workspacepkg.SkillRef)
	indirectByPath := make(map[string]workspacepkg.SkillRef)
	selectedByPath := make(map[string]workspacepkg.SkillRef)
	reasons := make([]workspacepkg.SkillScopeReason, 0, len(discovery.Skills))
	caveats := make([]string, 0, 8)
	ambiguous := make([]string, 0, 4)

	skillRefs := skillReferenceIndex(discovery, discovery.WorkspaceRoot)
	skillByRelativePath := make(map[string]workspacepkg.SkillRef, len(discovery.Skills))
	for _, skill := range discovery.Skills {
		relativeSkillPath, err := filepath.Rel(discovery.WorkspaceRoot, skill.Path)
		if err != nil {
			continue
		}
		skillByRelativePath[filepath.ToSlash(relativeSkillPath)] = skill
	}

	for _, changedFile := range changedFiles {
		matchedDirect := false
		for _, skill := range discovery.Skills {
			skillRelativePath, err := filepath.Rel(discovery.WorkspaceRoot, skill.Path)
			if err != nil {
				continue
			}
			skillRelativePath = filepath.ToSlash(skillRelativePath)
			if changedFile == skillRelativePath || strings.HasPrefix(changedFile, skillRelativePath+"/") {
				directByPath[skill.Path] = skill
				selectedByPath[skill.Path] = skill
				reasons = append(reasons, workspacepkg.SkillScopeReason{
					Skill:   skill,
					Impact:  workspacepkg.ImpactDirect,
					Summary: fmt.Sprintf("direct change under %s", skillRelativePath),
					Files:   []string{changedFile},
				})
				matchedDirect = true
			}
		}
		if matchedDirect {
			continue
		}

		referencedBy := referencedSkillsForFile(skillRefs, changedFile)
		if len(referencedBy) > 0 {
			for _, skill := range referencedBy {
				indirectByPath[skill.Path] = skill
				selectedByPath[skill.Path] = skill
				reasons = append(reasons, workspacepkg.SkillScopeReason{
					Skill:   skill,
					Impact:  workspacepkg.ImpactIndirect,
					Summary: fmt.Sprintf("shared resource change %s is referenced by the skill", changedFile),
					Files:   []string{changedFile},
				})
			}
			continue
		}

		if strings.HasSuffix(changedFile, "/"+workspaceSkillFileName) {
			caveats = append(caveats, fmt.Sprintf("changed skill entrypoint %s does not exist in the current workspace and may represent a deleted or moved skill", changedFile))
			ambiguous = append(ambiguous, changedFile)
			for _, skill := range discovery.Skills {
				selectedByPath[skill.Path] = skill
			}
			continue
		}

		if isAmbiguousWorkspaceChange(changedFile) {
			caveats = append(caveats, fmt.Sprintf("shared workspace file %s changed outside any single skill; all skills are included conservatively", changedFile))
			ambiguous = append(ambiguous, changedFile)
			for _, skill := range discovery.Skills {
				indirectByPath[skill.Path] = skill
				selectedByPath[skill.Path] = skill
			}
		}
	}

	direct := skillRefsFromMap(directByPath)
	indirect := skillRefsFromMap(indirectByPath)
	selected := skillRefsFromMap(selectedByPath)
	unchanged := make([]workspacepkg.SkillRef, 0, len(discovery.Skills))
	for _, skill := range discovery.Skills {
		if _, ok := selectedByPath[skill.Path]; ok {
			continue
		}
		unchanged = append(unchanged, skill)
	}

	return workspacepkg.BuildChangeScope(
		discovery.WorkspaceRoot,
		diffContext,
		changedFiles,
		direct,
		indirect,
		unchanged,
		selected,
		ambiguous,
		caveats,
		reasons,
	), nil
}

func workspaceChangedFiles(root string, options WorkspaceChangeOptions) ([]string, workspacepkg.DiffContext, error) {
	if strings.TrimSpace(options.BaseRev) == "" {
		tracked, err := runGitLines(root, "diff", "--name-only", "HEAD", "--")
		if err != nil {
			return nil, workspacepkg.DiffContext{}, err
		}
		untracked, err := runGitLines(root, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return nil, workspacepkg.DiffContext{}, err
		}
		return workspaceUniqueSortedStrings(append(tracked, untracked...)), workspacepkg.DiffContext{
			Mode:    workspacepkg.DiffModeWorkingTreeVsHEAD,
			HeadRev: "HEAD",
			Summary: "working tree changes relative to HEAD",
		}, nil
	}

	head := options.HeadRev
	if strings.TrimSpace(head) == "" {
		head = "HEAD"
	}
	files, err := runGitLines(root, "diff", "--name-only", options.BaseRev+".."+head, "--")
	if err != nil {
		return nil, workspacepkg.DiffContext{}, err
	}
	return workspaceUniqueSortedStrings(files), workspacepkg.DiffContext{
		Mode:    workspacepkg.DiffModeRevisionRange,
		BaseRev: options.BaseRev,
		HeadRev: head,
		Summary: fmt.Sprintf("git diff %s..%s", options.BaseRev, head),
	}, nil
}

func runGitLines(root string, args ...string) ([]string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}

	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	lines := make([]string, 0, 16)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, filepath.ToSlash(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func skillReferenceIndex(discovery workspacepkg.Discovery, workspaceRoot string) map[string][]workspacepkg.SkillRef {
	index := make(map[string][]workspacepkg.SkillRef)
	for _, skill := range discovery.Skills {
		references, err := localSkillReferences(skill.Path, workspaceRoot)
		if err != nil {
			continue
		}
		for _, reference := range references {
			index[reference] = append(index[reference], skill)
		}
	}
	return index
}

func localSkillReferences(skillPath, workspaceRoot string) ([]string, error) {
	content, err := os.ReadFile(filepath.Join(skillPath, workspaceSkillFileName))
	if err != nil {
		return nil, err
	}

	references := make([]string, 0, 8)
	appendRef := func(raw string) {
		candidate := strings.TrimSpace(raw)
		if candidate == "" ||
			strings.HasPrefix(candidate, "http://") ||
			strings.HasPrefix(candidate, "https://") ||
			strings.HasPrefix(candidate, "#") ||
			strings.HasPrefix(candidate, "/") {
			return
		}
		candidate = strings.Split(candidate, "#")[0]
		candidate = strings.Split(candidate, "?")[0]
		if candidate == "" {
			return
		}
		absolute := filepath.Clean(filepath.Join(skillPath, filepath.FromSlash(candidate)))
		relative, err := filepath.Rel(workspaceRoot, absolute)
		if err != nil || strings.HasPrefix(relative, "..") {
			return
		}
		references = append(references, filepath.ToSlash(relative))
	}

	for _, match := range workspaceMarkdownLinkPattern.FindAllStringSubmatch(string(content), -1) {
		if len(match) > 1 {
			appendRef(match[1])
		}
	}
	for _, match := range workspaceCodePathPattern.FindAllStringSubmatch(string(content), -1) {
		if len(match) > 1 {
			appendRef(match[1])
		}
	}

	return workspaceUniqueSortedStrings(references), nil
}

func referencedSkillsForFile(index map[string][]workspacepkg.SkillRef, changedFile string) []workspacepkg.SkillRef {
	values := index[changedFile]
	if len(values) == 0 {
		return nil
	}
	out := append([]workspacepkg.SkillRef(nil), values...)
	slices.SortFunc(out, func(a, b workspacepkg.SkillRef) int {
		return strings.Compare(a.Path, b.Path)
	})
	return out
}

func isAmbiguousWorkspaceChange(path string) bool {
	switch {
	case path == "README.md" || path == "README":
		return true
	case strings.HasPrefix(path, "docs/"):
		return true
	case strings.HasPrefix(path, "shared/"):
		return true
	case strings.HasPrefix(path, "templates/"):
		return true
	case strings.HasPrefix(path, "assets/"):
		return true
	case strings.HasPrefix(path, "examples/"):
		return true
	case strings.HasPrefix(path, "scripts/"):
		return true
	default:
		return false
	}
}

func skillRefsFromMap(values map[string]workspacepkg.SkillRef) []workspacepkg.SkillRef {
	out := make([]workspacepkg.SkillRef, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	slices.SortFunc(out, func(a, b workspacepkg.SkillRef) int {
		return strings.Compare(a.Path, b.Path)
	})
	return out
}

func workspaceUniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	return out
}
