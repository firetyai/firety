package main

import (
	"fmt"
	"os"

	"github.com/firety/firety/internal/domain/lint"
)

func main() {
	if _, err := fmt.Fprint(os.Stdout, lint.MarkdownCatalog()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
