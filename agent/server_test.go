package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	themes := filepath.Join(dir, "themes")
	for _, d := range []string{web, themes} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(web, "index.html"), "<html>stremio</html>")
	write(filepath.Join(web, "service-worker.js"), "self.skipWaiting()")
	write(filepath.Join(themes, "default.css"), ":root{}")
	write(filepath.Join(dir, "addons.json"), `{"addons":[{"url":"https://a.example/{{K}}/manifest.json"}]}`)
	write(filepath.Join(dir, "secrets.env"), "K=s3cret\n")

	var nmcli []string
	s := &server{
		wifi: &Wifi{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			nmcli = append(nmcli, strings.Join(args, " "))
			return []byte("*:Home:70:WPA2\n"), nil
		}},
		online:      NewOnline("http://127.0.0.1:1/unreachable"),
		addonsPath:  filepath.Join(dir, "addons.json"),
		secretsPath: filepath.Join(dir, "secrets.env"),
		version:     "test",
	}
	ts := httptest.NewServer(s.routes(web, themes))
	t.Cleanup(ts.Close)
	return ts, &nmcli
}

func get(t *testing.T, ts *httptest.Server, path string) (*http.Response, string) {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func TestRoutes(t *testing.T) {
	ts, _ := testServer(t)
	cases := []struct {
		path, want string
		status     int
	}{
		{"/", "stremio", 200},
		{"/weebio/boot", "Connecting to the internet", 200},
		{"/weebio/setup", "Connect to Wi-Fi", 200},
		{"/weebio/common.js", "spatialNavigation", 200},
		{"/weebio/themes/default.css", ":root", 200},
		{"/weebio/api/version", `"test"`, 200},
		{"/weebio/api/net", `"online":false`, 200},
		{"/weebio/api/wifi/networks", `"ssid":"Home"`, 200},
		{"/service-worker.js", "", 404},
	}
	for _, c := range cases {
		resp, body := get(t, ts, c.path)
		if resp.StatusCode != c.status || !strings.Contains(body, c.want) {
			t.Errorf("GET %s: status %d, body %.80q; want %d containing %q", c.path, resp.StatusCode, body, c.status, c.want)
		}
	}
	if resp, _ := get(t, ts, "/weebio/"); resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/weebio/boot" {
		t.Errorf("GET /weebio/: %d -> %q, want redirect to /weebio/boot", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestAddonsEndpointRendersSecretsWithoutCaching(t *testing.T) {
	ts, _ := testServer(t)
	resp, body := get(t, ts, "/weebio/api/addons")
	var list AddonList
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	if len(list.Addons) != 1 || list.Addons[0].URL != "https://a.example/s3cret/manifest.json" {
		t.Fatalf("got %+v", list)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", resp.Header.Get("Cache-Control"))
	}
}

func TestConnectRequiresHeader(t *testing.T) {
	ts, nmcli := testServer(t)
	post := func(header bool) int {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/weebio/api/wifi/connect", strings.NewReader(`{"ssid":"Home","password":"pw"}`))
		if header {
			req.Header.Set("X-Weebio", "1")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := post(false); code != http.StatusForbidden {
		t.Fatalf("without header: %d, want 403", code)
	}
	if len(*nmcli) != 0 {
		t.Fatalf("nmcli ran without the header: %q", *nmcli)
	}
	if code := post(true); code != http.StatusOK {
		t.Fatalf("with header: %d, want 200", code)
	}
	if last := (*nmcli)[len(*nmcli)-1]; !strings.Contains(last, "connect Home password pw") {
		t.Fatalf("nmcli args %q", last)
	}
}
