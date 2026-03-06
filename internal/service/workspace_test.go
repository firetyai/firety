package service_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/domain/readiness"
	workspacepkg "github.com/firety/firety/internal/domain/workspace"
	"github.com/firety/firety/internal/service"
	"github.com/firety/firety/internal/testutil"
)

func TestWorkspaceAnalyzeDiscoversAndSortsSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"skills/bravo/SKILL.md":          testutil.ValidSkillFiles()["SKILL.md"],
		"skills/bravo/docs/reference.md": testutil.ValidSkillFiles()["docs/reference.md"],
		"skills/alpha/SKILL.md":          testutil.ValidSkillFiles()["SKILL.md"],
		"skills/alpha/docs/reference.md": testutil.ValidSkillFiles()["docs/reference.md"],
	})

	report, err := service.NewWorkspaceService(service.NewSkillLinter(), newTestReadinessService()).Analyze(root, service.WorkspaceAnalyzeOptions{
		Profile:    service.SkillLintProfileGeneric,
		Strictness: lint.StrictnessDefault,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(report.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %#v", report.Skills)
	}
	if !strings.HasSuffix(report.Skills[0].Skill.Path, "/skills/alpha") {
		t.Fatalf("expected alpha to sort first, got %#v", report.Skills)
	}
	if report.Summary.CleanSkills != 2 {
		t.Fatalf("expected two clean skills, got %#v", report.Summary)
	}
}

func TestWorkspaceAnalyzeReportsDiscoveryWarnings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"skill-a/SKILL.md":          testutil.ValidSkillFiles()["SKILL.md"],
		"skill-a/docs/reference.md": testutil.ValidSkillFiles()["docs/reference.md"],
	})
	if err := os.Mkdir(root+"/broken", 0o000); err != nil {
		t.Fatalf("expected to create broken dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root+"/broken", 0o755) })

	report, err := service.NewWorkspaceService(service.NewSkillLinter(), newTestReadinessService()).Analyze(root, service.WorkspaceAnalyzeOptions{
		Profile:    service.SkillLintProfileGeneric,
		Strictness: lint.StrictnessDefault,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(report.Discovery.Warnings) == 0 {
		t.Fatalf("expected discovery warning, got %#v", report.Discovery)
	}
}

func TestWorkspaceAnalyzeIncludesReadinessAndGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"healthy/SKILL.md":          testutil.ValidSkillFiles()["SKILL.md"],
		"healthy/docs/reference.md": testutil.ValidSkillFiles()["docs/reference.md"],
		"tiny/SKILL.md":             "# Tiny\n",
	})

	report, err := service.NewWorkspaceService(service.NewSkillLinter(), newTestReadinessService()).Analyze(root, service.WorkspaceAnalyzeOptions{
		Profile:          service.SkillLintProfileGeneric,
		Strictness:       lint.StrictnessDefault,
		IncludeReadiness: true,
		ReadinessContext: readiness.ContextPublicRelease,
		GateCriteria: &workspacepkg.GateCriteria{
			MaxNotReadySkills:             0,
			MaxInsufficientEvidenceSkills: 0,
			MaxSkillsWithLintErrors:       0,
			MaxDiscoveryWarnings:          0,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Gate == nil {
		t.Fatalf("expected gate result")
	}
	if report.Gate.Decision == "" {
		t.Fatalf("expected gate decision, got %#v", report.Gate)
	}
	if report.Summary.SkillCount != 2 {
		t.Fatalf("expected 2 skills, got %#v", report.Summary)
	}
	if report.Summary.ReadySkills+report.Summary.ReadyWithCaveatsSkills+report.Summary.NotReadySkills+report.Summary.InsufficientEvidenceSkills == 0 {
		t.Fatalf("expected readiness summary, got %#v", report.Summary)
	}
}

func TestWorkspaceChangesDetectsDirectSkillChange(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"skills/alpha/SKILL.md":          testutil.ValidSkillFiles()["SKILL.md"],
		"skills/alpha/docs/reference.md": testutil.ValidSkillFiles()["docs/reference.md"],
		"skills/bravo/SKILL.md":          testutil.ValidSkillFiles()["SKILL.md"],
		"skills/bravo/docs/reference.md": testutil.ValidSkillFiles()["docs/reference.md"],
	})
	initGitRepo(t, root)

	testutil.WriteFiles(t, root, map[string]string{
		"skills/alpha/SKILL.md": testutil.ValidSkillFiles()["SKILL.md"] + "\nextra line\n",
	})

	scope, err := service.NewWorkspaceService(service.NewSkillLinter(), newTestReadinessService()).Changes(root, service.WorkspaceChangeOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(scope.DirectlyChangedSkills) != 1 || scope.DirectlyChangedSkills[0].Name != "alpha" {
		t.Fatalf("expected only alpha to be directly changed, got %#v", scope.DirectlyChangedSkills)
	}
	if len(scope.SelectedSkills) != 1 || scope.SelectedSkills[0].Name != "alpha" {
		t.Fatalf("expected only alpha in selected scope, got %#v", scope.SelectedSkills)
	}
	if len(scope.UnchangedSkills) != 1 || scope.UnchangedSkills[0].Name != "bravo" {
		t.Fatalf("expected bravo to remain unchanged, got %#v", scope.UnchangedSkills)
	}
}

func TestWorkspaceChangesDetectsSharedFileImpact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"shared/guide.md": "# Shared\n",
		"skills/alpha/SKILL.md": `# Alpha

See [Guide](../../shared/guide.md)
`,
		"skills/bravo/SKILL.md": "# Bravo\n",
	})
	initGitRepo(t, root)

	testutil.WriteFiles(t, root, map[string]string{
		"shared/guide.md": "# Shared\n\nUpdated.\n",
	})

	scope, err := service.NewWorkspaceService(service.NewSkillLinter(), newTestReadinessService()).Changes(root, service.WorkspaceChangeOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(scope.DirectlyChangedSkills) != 0 {
		t.Fatalf("expected no direct skill changes, got %#v", scope.DirectlyChangedSkills)
	}
	if len(scope.ImpactedSkills) != 1 || scope.ImpactedSkills[0].Name != "alpha" {
		t.Fatalf("expected alpha to be impacted, got %#v", scope.ImpactedSkills)
	}
}

func TestWorkspaceChangesSurfacesDeletedSkillCaveat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"skills/alpha/SKILL.md": "# Alpha\n",
		"skills/bravo/SKILL.md": "# Bravo\n",
	})
	initGitRepo(t, root)

	if err := os.RemoveAll(filepath.Join(root, "skills/bravo")); err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}

	scope, err := service.NewWorkspaceService(service.NewSkillLinter(), newTestReadinessService()).Changes(root, service.WorkspaceChangeOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(scope.Caveats) == 0 {
		t.Fatalf("expected deleted-skill caveat, got %#v", scope)
	}
	if len(scope.SelectedSkills) != 1 || scope.SelectedSkills[0].Name != "alpha" {
		t.Fatalf("expected remaining workspace skill to be selected conservatively, got %#v", scope.SelectedSkills)
	}
}

func newTestReadinessService() service.SkillReadinessService {
	return app.New(app.VersionInfo{}).Services.SkillReadiness
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
