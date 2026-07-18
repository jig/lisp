package batsyntax

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallIdempotent(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	if err := Install(dir, &out); err != nil {
		t.Fatal(err)
	}
	syntax, err := os.ReadFile(filepath.Join(dir, "syntaxes", syntaxFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(syntax, []byte("name: jig/lisp")) {
		t.Error("syntax file lacks the jig/lisp name")
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(cfg), mapSyntaxLine); got != 1 {
		t.Fatalf("config has %d map-syntax lines, want 1:\n%s", got, cfg)
	}

	// second run must not duplicate the mapping
	if err := Install(dir, &out); err != nil {
		t.Fatal(err)
	}
	cfg, _ = os.ReadFile(filepath.Join(dir, "config"))
	if got := strings.Count(string(cfg), mapSyntaxLine); got != 1 {
		t.Fatalf("after reinstall, config has %d map-syntax lines, want 1", got)
	}
}

func TestInstallAppendsToExistingConfig(t *testing.T) {
	dir := t.TempDir()
	// existing config without a trailing newline
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(`--theme="ansi"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	want := "--theme=\"ansi\"\n" + mapSyntaxLine + "\n"
	if string(cfg) != want {
		t.Errorf("config = %q, want %q", cfg, want)
	}
}

func TestConfigDirEnvOverride(t *testing.T) {
	t.Setenv("BAT_CONFIG_DIR", "/custom/bat")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/custom/bat" {
		t.Errorf("ConfigDir() = %q, want /custom/bat", dir)
	}
}
