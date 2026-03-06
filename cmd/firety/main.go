package main

import (
	"fmt"
	"os"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/cli"
	"github.com/firety/firety/internal/platform/buildinfo"
)

func main() {
	metadata := buildinfo.Current()

	application := app.New(app.VersionInfo{
		Version: metadata.Version,
		Commit:  metadata.Commit,
		Date:    metadata.Date,
	})

	exitCode, err := cli.Execute(application, os.Stdout, os.Stderr)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}

	os.Exit(exitCode)
}
