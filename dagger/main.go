package main

import (
	"dagger/lisp/internal/dagger"
)

type Lisp struct{}

// Setup cached dependencies
func (m *Lisp) Setup(source *dagger.Directory) *dagger.Container {
	return dag.Container().
		From("golang:1.24").
		WithDirectory("/lisp", source).
		WithWorkdir("/lisp").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod-124")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build-124")).
		WithEnvVariable("GOCACHE", "/go/build-cache")
}

// Build lisp interpreter
func (m *Lisp) Build(source *dagger.Directory) *dagger.Container {
	return m.Setup(source).
		WithExec([]string{"go", "build", "-o", "/lisp/lisp", "cmd/lisp/main.go"})
}

// Execute all tests
func (m *Lisp) Test(source *dagger.Directory) *dagger.Container {
	return m.Setup(source).
		WithExec([]string{"go", "test", "./..."})
}

// Execute all benchmarks
func (m *Lisp) Benchmark(source *dagger.Directory) *dagger.Container {
	return m.Setup(source).
		WithExec([]string{"go", "test", "--bench=.", "./..."})
}

// Lint the source code
func (m *Lisp) Lint(source *dagger.Directory) *dagger.Container {
	return m.Setup(source).
		WithExec([]string{"golangci-lint", "run", source.Name()})
}
