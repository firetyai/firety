package app

import "github.com/firety/firety/internal/service"

type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

type Services struct {
	Placeholder      service.PlaceholderService
	SkillLint        service.SkillLinter
	SkillFix         service.SkillFixer
	SkillCompare     service.SkillCompareService
	SkillEval        service.SkillEvalService
	SkillEvalCompare service.SkillEvalCompareService
	SkillAnalyze     service.SkillAnalyzeService
	SkillPlan        service.SkillPlanService
	Benchmark        service.BenchmarkService
}

type App struct {
	Version  VersionInfo
	Services Services
}

func New(version VersionInfo) *App {
	skillLint := service.NewSkillLinter()
	skillEval := service.NewSkillEvalService()

	return &App{
		Version: version,
		Services: Services{
			Placeholder:      service.NewPlaceholderService(),
			SkillLint:        skillLint,
			SkillFix:         service.NewSkillFixer(),
			SkillCompare:     service.NewSkillCompareService(skillLint),
			SkillEval:        skillEval,
			SkillEvalCompare: service.NewSkillEvalCompareService(skillEval),
			SkillAnalyze:     service.NewSkillAnalyzeService(skillLint, skillEval),
			SkillPlan:        service.NewSkillPlanService(skillLint, skillEval),
			Benchmark:        service.NewBenchmarkService(skillLint),
		},
	}
}
