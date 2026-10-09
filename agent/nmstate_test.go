package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseGeneralState(t *testing.T) {
	for in, want := range map[string]string{
		"connected\n":              "connected",
		"connected (local only)\n": "limited",
		"connected (site only)":    "limited",
		"connecting\n":             "connecting",
		"disconnected\n":           "disconnected",
		"asleep\n":                 "disconnected",
		"":                         "unknown",
	} {
		if got := parseGeneralState(in); got != want {
			t.Errorf("parseGeneralState(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCountWifiProfiles(t *testing.T) {
	out := "802-11-wireless\nloopback\n802-3-ethernet\n802-11-wireless\n"
	if got := countWifiProfiles(out); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}

func TestNetworkManagerStateWithoutNmcli(t *testing.T) {
	nm := &NetworkManager{run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("nmcli: not found")
	}}
	if st := nm.State(context.Background()); st.State != "unknown" || st.SavedWifi != 0 {
		t.Fatalf("got %+v", st)
	}
}

func TestNetworkManagerState(t *testing.T) {
	nm := &NetworkManager{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "general") {
			return []byte("connecting\n"), nil
		}
		return []byte("802-11-wireless\nloopback\n"), nil
	}}
	if st := nm.State(context.Background()); st.State != "connecting" || st.SavedWifi != 1 {
		t.Fatalf("got %+v", st)
	}
}

func TestParseVia(t *testing.T) {
	for out, want := range map[string]string{
		"ethernet:connected\nwifi:connected\nloopback:connected (externally)\n":    "ethernet",
		"wifi:connected\nethernet:unavailable\n":                                   "wifi",
		"wifi:connected\nethernet:connected\n":                                     "ethernet",
		"wifi:disconnected\nethernet:unavailable\nloopback:connected (externally)": "",
		"": "",
	} {
		if got := parseVia(out); got != want {
			t.Errorf("parseVia(%q) = %q, want %q", out, got, want)
		}
	}
}
