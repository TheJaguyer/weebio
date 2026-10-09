package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
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

// addonSpec is one entry of addons.json.
type addonSpec struct {
	URL string `json:"url"`
	// Config is the addon's settings as readable JSON. It is rendered (placeholders filled), compacted,
	// base64-encoded and substituted for "{config}" in URL: the form most configurable addons use.
	Config json.RawMessage `json:"config,omitempty"`
	// Remove names parts of the addon to strip ("catalogs", "search", "meta", ...); applied by the UI.
	Remove []string `json:"remove,omitempty"`
}

type Addon struct {
	URL    string   `json:"url"`
	Remove []string `json:"remove,omitempty"`
}

type AddonList struct {
	Addons   []Addon  `json:"addons"`
	Warnings []string `json:"warnings"`
}

// Placeholders: {{KEY}}, filled from the secrets file. Inside "config" they belong inside JSON strings.
var placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// fill replaces placeholders in text, escaping values with escape and recording missing keys.
func fill(text string, secrets map[string]string, escape func(string) string, missing *[]string) string {
	return placeholder.ReplaceAllStringFunc(text, func(m string) string {
		key := placeholder.FindStringSubmatch(m)[1]
		v, ok := secrets[key]
		if !ok {
			*missing = append(*missing, key)
		}
		return escape(v)
	})
}

// jsonStringContent escapes v for use inside a JSON string literal (without the quotes).
func jsonStringContent(v string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // match JavaScript's JSON.stringify, which addons' config pages use
	_ = enc.Encode(v)
	out := strings.TrimSuffix(buf.String(), "\n")
	return out[1 : len(out)-1]
}

func identity(s string) string { return s }

// renderAddons turns addons.json into installable URLs. An addon whose keys aren't all present in the
// secrets file is left out with a warning rather than installed with a broken URL.
func renderAddons(raw []byte, secrets map[string]string) (AddonList, error) {
	var in struct {
		Addons []addonSpec `json:"addons"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return AddonList{}, fmt.Errorf("addons.json: %w", err)
	}
	out := AddonList{Addons: []Addon{}, Warnings: []string{}}
	for _, a := range in.Addons {
		var missing []string
		url := fill(a.URL, secrets, identity, &missing)
		if len(a.Config) > 0 {
			var compact bytes.Buffer
			if err := json.Compact(&compact, []byte(fill(string(a.Config), secrets, jsonStringContent, &missing))); err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("skipped %s: invalid config: %v", redact(a.URL), err))
				continue
			}
			url = strings.ReplaceAll(url, "{config}", base64.StdEncoding.EncodeToString(compact.Bytes()))
		}
		if len(missing) > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("skipped %s: missing %s in secrets file", redact(a.URL), strings.Join(missing, ", ")))
			continue
		}
		out.Addons = append(out.Addons, Addon{URL: url, Remove: a.Remove})
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
