// Package batsyntax installs the jig/lisp syntax definition into bat
// (https://github.com/sharkdp/bat), so `bat file.lisp` highlights lisp
// source with the same grammar the VS Code extension uses. The syntax
// file is embedded, which makes any lisp binary a self-contained
// installer (`lisp --install-bat-syntax`) — handy on machines that have
// the binary but not the repository.
package batsyntax

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "embed"
)

//go:embed jig-lisp.sublime-syntax
var syntaxFile []byte

const (
	syntaxFileName = "jig-lisp.sublime-syntax"
	// mapSyntaxLine claims the .lisp extension explicitly: bat bundles
	// its own "Lisp" syntax for that extension, and the config mapping
	// is the deterministic way to override it.
	mapSyntaxLine = `--map-syntax "*.lisp:jig/lisp"`
)

// ConfigDir resolves bat's configuration directory the way bat does:
// $BAT_CONFIG_DIR first, then `bat --config-dir`, then ~/.config/bat.
func ConfigDir() (string, error) {
	if dir := os.Getenv("BAT_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	if _, err := exec.LookPath("bat"); err == nil {
		out, err := exec.Command("bat", "--config-dir").Output()
		if err == nil {
			if dir := strings.TrimSpace(string(out)); dir != "" {
				return dir, nil
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve bat's config dir: %w", err)
	}
	return filepath.Join(home, ".config", "bat"), nil
}

// Install writes the syntax file into dir/syntaxes and ensures the
// map-syntax override in dir/config. It is idempotent.
func Install(dir string, w io.Writer) error {
	syntaxDir := filepath.Join(dir, "syntaxes")
	if err := os.MkdirAll(syntaxDir, 0o755); err != nil {
		return err
	}
	syntaxPath := filepath.Join(syntaxDir, syntaxFileName)
	if err := os.WriteFile(syntaxPath, syntaxFile, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(w, "wrote %s\n", syntaxPath)

	configPath := filepath.Join(dir, "config")
	current, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(current), mapSyntaxLine) {
		fmt.Fprintf(w, "%s already maps *.lisp\n", configPath)
		return nil
	}
	updated := string(current)
	if updated != "" && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	updated += mapSyntaxLine + "\n"
	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(w, "added %s to %s\n", mapSyntaxLine, configPath)
	return nil
}

// BuildCache rebuilds bat's syntax cache so the new definition takes
// effect. When bat is not on PATH it only prints the pending step.
func BuildCache(w io.Writer) error {
	if _, err := exec.LookPath("bat"); err != nil {
		fmt.Fprintln(w, "bat not found on PATH; run `bat cache --build` once it is installed")
		return nil
	}
	cmd := exec.Command("bat", "cache", "--build")
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bat cache --build: %w", err)
	}
	return nil
}
