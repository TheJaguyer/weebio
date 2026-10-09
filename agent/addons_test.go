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

// The real addons.json shape, with dummy keys. Expected URLs were produced independently with
// JavaScript (JSON.stringify + base64), i.e. exactly what the addons' own config pages generate.
func TestRenderAddonsConfig(t *testing.T) {
	raw := []byte(`{"addons": [
		{
			"url": "https://addon.debridio.com/{config}/manifest.json",
			"config": {
				"api_key": "{{DEBRIDIO_API_KEY}}",
				"provider": "torbox",
				"providerKey": "{{TORBOX_API_KEY}}",
				"disableUncached": true,
				"maxSize": "",
				"maxReturnPerQuality": 6,
				"resolutions": ["4k", "1440p", "1080p", "720p", "unknown"],
				"excludedQualities": ["TVRip", "R5", "PPVRip", "TeleCine", "TeleSync", "SCR", "CAM"],
				"preferredLang": ["english"]
			}
		},
		{
			"url": "https://opensubtitlesv3-pro.dexter21767.com/{config}/manifest.json",
			"config": { "langs": ["english", "korean"], "source": "all", "aiTranslated": false, "autoAdjustment": true }
		},
		{ "url": "https://v3-cinemeta.strem.io/manifest.json", "remove": ["catalogs", "search", "meta"] },
		{ "url": "https://tmdb.example/{config}/manifest.json", "config": { "api_key": "{{NOT_SET}}" } }
	]}`)
	got, err := renderAddons(raw, map[string]string{"DEBRIDIO_API_KEY": "DUMMYDEBRIDIOKEY", "TORBOX_API_KEY": "DUMMY-TORBOX"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Addon{
		{URL: "https://addon.debridio.com/eyJhcGlfa2V5IjoiRFVNTVlERUJSSURJT0tFWSIsInByb3ZpZGVyIjoidG9yYm94IiwicHJvdmlkZXJLZXkiOiJEVU1NWS1UT1JCT1giLCJkaXNhYmxlVW5jYWNoZWQiOnRydWUsIm1heFNpemUiOiIiLCJtYXhSZXR1cm5QZXJRdWFsaXR5Ijo2LCJyZXNvbHV0aW9ucyI6WyI0ayIsIjE0NDBwIiwiMTA4MHAiLCI3MjBwIiwidW5rbm93biJdLCJleGNsdWRlZFF1YWxpdGllcyI6WyJUVlJpcCIsIlI1IiwiUFBWUmlwIiwiVGVsZUNpbmUiLCJUZWxlU3luYyIsIlNDUiIsIkNBTSJdLCJwcmVmZXJyZWRMYW5nIjpbImVuZ2xpc2giXX0=/manifest.json"},
		{URL: "https://opensubtitlesv3-pro.dexter21767.com/eyJsYW5ncyI6WyJlbmdsaXNoIiwia29yZWFuIl0sInNvdXJjZSI6ImFsbCIsImFpVHJhbnNsYXRlZCI6ZmFsc2UsImF1dG9BZGp1c3RtZW50Ijp0cnVlfQ==/manifest.json"},
		{URL: "https://v3-cinemeta.strem.io/manifest.json", Remove: []string{"catalogs", "search", "meta"}},
	}
	if !reflect.DeepEqual(got.Addons, want) {
		t.Fatalf("addons:\n got %+v\nwant %+v", got.Addons, want)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "NOT_SET") {
		t.Fatalf("warnings: %q", got.Warnings)
	}
}

func TestJSONStringContentEscapes(t *testing.T) {
	if got := jsonStringContent(`a"b\c<d>`); got != `a\"b\\c<d>` {
		t.Fatalf("got %q", got)
	}
}
