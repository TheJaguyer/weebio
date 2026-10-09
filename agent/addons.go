package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// parseSecrets reads KEY=value lines (blank lines and # comments ignored, optional surrounding quotes).
func parseSecrets(r io.Reader) (map[string]string, error) {
	secrets := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if value != "" {
			secrets[key] = value
		}
	}
	return secrets, sc.Err()
}

func loadSecrets(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil // no secrets file: keyed addons are simply skipped
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseSecrets(f)
}

type Addon struct {
	URL string `json:"url"`
}

type AddonList struct {
	Addons   []Addon  `json:"addons"`
	Warnings []string `json:"warnings"`
}

var placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// renderAddons substitutes {{KEY}} placeholders. An addon whose keys aren't all present is
// left out with a warning rather than installed with a broken URL.
func renderAddons(raw []byte, secrets map[string]string) (AddonList, error) {
	var in struct {
		Addons []Addon `json:"addons"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return AddonList{}, fmt.Errorf("addons.json: %w", err)
	}
	out := AddonList{Addons: []Addon{}, Warnings: []string{}}
	for _, a := range in.Addons {
		var missing []string
		url := placeholder.ReplaceAllStringFunc(a.URL, func(m string) string {
			key := placeholder.FindStringSubmatch(m)[1]
			v, ok := secrets[key]
			if !ok {
				missing = append(missing, key)
			}
			return v
		})
		if len(missing) > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("skipped %s: missing %s in secrets file", redact(a.URL), strings.Join(missing, ", ")))
			continue
		}
		out.Addons = append(out.Addons, Addon{URL: url})
	}
	return out, nil
}

// redact shows an addon's host only, so warnings never echo a URL that could carry other keys.
func redact(url string) string {
	rest := url
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return rest
}
