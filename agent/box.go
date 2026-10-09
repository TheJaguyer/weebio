package main

import (
	"os"
	"strings"
)

// elderFuthark lists the rune names a box can be identified by (Elder Futhark order).
var elderFuthark = []string{
	"fehu", "uruz", "thurisaz", "ansuz", "raidho", "kenaz", "gebo", "wunjo",
	"hagalaz", "naudiz", "isa", "jera", "eihwaz", "perthro", "algiz", "sowilo",
	"tiwaz", "berkano", "ehwaz", "mannaz", "laguz", "ingwaz", "dagaz", "othala",
}

// normalizeRune returns the canonical rune name in s, or "" if s doesn't name one.
// Accepts "Fehu", "fehu", or a hostname like "saga-fehu".
func normalizeRune(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.LastIndexAny(s, "-_."); i >= 0 {
		s = s[i+1:]
	}
	for _, name := range elderFuthark {
		if s == name {
			return name
		}
	}
	return ""
}

// BoxIdentity is what the UI shows so a friend can tell you which box they have.
type BoxIdentity struct {
	Rune     string `json:"rune"` // "" when the box has no rune configured
	Hostname string `json:"hostname"`
}

// boxIdentity reads RUNE= from the box config file (next to the secrets), falling back to the hostname.
func boxIdentity(configPath string) BoxIdentity {
	host, _ := os.Hostname()
	id := BoxIdentity{Hostname: host}
	if f, err := os.Open(configPath); err == nil {
		defer f.Close()
		if values, err := parseSecrets(f); err == nil {
			id.Rune = normalizeRune(values["RUNE"])
		}
	}
	if id.Rune == "" {
		id.Rune = normalizeRune(host)
	}
	return id
}
