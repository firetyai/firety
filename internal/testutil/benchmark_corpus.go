package testutil

import "github.com/firety/firety/internal/benchmark"

type BenchmarkSkillFixture = benchmark.BenchmarkSkillFixture
type BenchmarkExpectations = benchmark.BenchmarkExpectations

func SkillLintBenchmarkCorpus() []BenchmarkSkillFixture {
	return benchmark.SkillLintBenchmarkCorpus()
}
