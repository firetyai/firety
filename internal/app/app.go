package app

import "github.com/firety/firety/internal/service"

type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

type Services struct {
	Placeholder        service.PlaceholderService
	SkillLint          service.SkillLinter
	SkillFix           service.SkillFixer
	SkillCompare       service.SkillCompareService
	SkillEval          service.SkillEvalService
	SkillEvalCompare   service.SkillEvalCompareService
	SkillAnalyze       service.SkillAnalyzeService
	SkillCompatibility service.SkillCompatibilityService
	SkillPlan          service.SkillPlanService
	SkillGate          service.SkillGateService
	SkillReadiness     service.SkillReadinessService
	SkillBaseline      service.SkillBaselineService
	SkillAttest        service.SkillAttestService
	Benchmark          service.BenchmarkService
}

type App struct {
	Version  VersionInfo
	Services Services
}

func New(version VersionInfo) *App {
	skillLint := service.NewSkillLinter()
	skillEval := service.NewSkillEvalService()
	skillCompare := service.NewSkillCompareService(skillLint)
	skillEvalCompare := service.NewSkillEvalCompareService(skillEval)
	skillCompatibility := service.NewSkillCompatibilityService(skillLint, skillEval)
	skillGate := service.NewSkillGateService(skillLint, skillCompare, skillEval, skillEvalCompare)
	skillAttest := service.NewSkillAttestService(skillCompatibility, skillGate, skillEval)
	skillReadiness := service.NewSkillReadinessService(skillCompatibility, skillGate, skillAttest)

	return &App{
		Version: version,
		Services: Services{
			Placeholder:        service.NewPlaceholderService(),
			SkillLint:          skillLint,
			SkillFix:           service.NewSkillFixer(),
			SkillCompare:       skillCompare,
			SkillEval:          skillEval,
			SkillEvalCompare:   skillEvalCompare,
			SkillAnalyze:       service.NewSkillAnalyzeService(skillLint, skillEval),
			SkillCompatibility: skillCompatibility,
			SkillPlan:          service.NewSkillPlanService(skillLint, skillEval),
			SkillGate:          skillGate,
			SkillReadiness:     skillReadiness,
			SkillBaseline:      service.NewSkillBaselineService(skillLint, skillEval),
			SkillAttest:        skillAttest,
			Benchmark:          service.NewBenchmarkService(skillLint),
		},
	}
}
