package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRune(t *testing.T) {
	for in, want := range map[string]string{
		"fehu":      "fehu",
		" Fehu ":    "fehu",
		"saga-uruz": "uruz",
		"box.ansuz": "ansuz",
		"OTHALA":    "othala",
		"weebio-01": "",
		"pi-alpha":  "",
		"":          "",
	} {
		if got := normalizeRune(in); got != want {
			t.Errorf("normalizeRune(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBoxIdentityPrefersConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "box.env")
	if err := os.WriteFile(path, []byte("# this box\nRUNE=Kenaz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := boxIdentity(path).Rune; got != "kenaz" {
		t.Fatalf("rune = %q, want kenaz", got)
	}
}

func TestBoxIdentityWithoutConfigUsesHostname(t *testing.T) {
	host, _ := os.Hostname()
	got := boxIdentity(filepath.Join(t.TempDir(), "missing.env"))
	if got.Hostname != host || got.Rune != normalizeRune(host) {
		t.Fatalf("got %+v for hostname %q", got, host)
	}
}

func TestElderFutharkHas24Runes(t *testing.T) {
	if len(elderFuthark) != 24 {
		t.Fatalf("got %d runes", len(elderFuthark))
	}
}
