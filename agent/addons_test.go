package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSecrets(t *testing.T) {
	got, err := parseSecrets(strings.NewReader(`
# comment
RD_KEY=abc123
export TB_KEY="quoted value"
EMPTY=
SPACED = trimmed
garbage line
`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"RD_KEY": "abc123", "TB_KEY": "quoted value", "SPACED": "trimmed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRenderAddons(t *testing.T) {
	raw := []byte(`{"addons":[
		{"url":"https://v3-cinemeta.strem.io/manifest.json"},
		{"url":"https://torrentio.strem.fun/realdebrid={{RD_KEY}}/manifest.json"},
		{"url":"https://other.example/{{ TB_KEY }}/manifest.json"}
	]}`)
	got, err := renderAddons(raw, map[string]string{"RD_KEY": "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	wantAddons := []Addon{
		{URL: "https://v3-cinemeta.strem.io/manifest.json"},
		{URL: "https://torrentio.strem.fun/realdebrid=abc123/manifest.json"},
	}
	if !reflect.DeepEqual(got.Addons, wantAddons) {
		t.Fatalf("addons: got %+v", got.Addons)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "other.example") || !strings.Contains(got.Warnings[0], "TB_KEY") {
		t.Fatalf("warnings: got %q", got.Warnings)
	}
	if strings.Contains(strings.Join(got.Warnings, " "), "abc123") {
		t.Fatal("warning leaked a secret")
	}
}
