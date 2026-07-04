package command

import (
	"fmt"
	"io"
	"os"

	"github.com/jig/lisp/format"
)

// formatSources formats each file in place (write) or to stdout. When no
// files are given it reads from stdin and always writes to stdout, since
// there is nothing to rewrite. This is the CLI counterpart of the LSP
// textDocument/formatting handler; both call format.Source.
func formatSources(files []string, write bool) error {
	if len(files) == 0 {
		if write {
			return fmt.Errorf("--fmt -w needs at least one file (cannot rewrite stdin)")
		}
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		out, err := format.Source(src)
		if err != nil {
			return fmt.Errorf("<stdin>: %w", err)
		}
		_, err = os.Stdout.Write(out)
		return err
	}

	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		out, err := format.Source(src)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if write {
			// Only touch the file when formatting actually changed it, to
			// preserve mtimes and play nicely with editors watching on save.
			if string(out) != string(src) {
				info, err := os.Stat(name)
				if err != nil {
					return err
				}
				if err := os.WriteFile(name, out, info.Mode().Perm()); err != nil {
					return err
				}
			}
		} else {
			if _, err := os.Stdout.Write(out); err != nil {
				return err
			}
		}
	}
	return nil
}
