package main

import (
	"context"
	"strings"
	"sync"
	"time"
)

// NMState is NetworkManager's view of connectivity, simplified for the boot page:
// "connected", "limited" (local/site only), "connecting", "disconnected" or "unknown".
type NMState struct {
	State     string `json:"state"`
	SavedWifi int    `json:"savedWifi"` // saved Wi-Fi profiles NetworkManager could try
	Via       string `json:"via"`       // "ethernet", "wifi" or "" — how the box is connected
}

// parseGeneralState maps `nmcli -t -f STATE general` output to NMState.State.
func parseGeneralState(out string) string {
	switch s := strings.TrimSpace(out); {
	case s == "connected":
		return "connected"
	case strings.HasPrefix(s, "connected"): // "connected (local only)", "connected (site only)"
		return "limited"
	case s == "connecting":
		return "connecting"
	case s == "disconnected", s == "disconnecting", s == "asleep":
		return "disconnected"
	default:
		return "unknown"
	}
}

// countWifiProfiles counts lines of `nmcli -t -f TYPE connection show` that are Wi-Fi.
func countWifiProfiles(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "802-11-wireless" {
			n++
		}
	}
	return n
}

// parseVia picks the connection type from `nmcli -t -f TYPE,STATE device`, preferring a wired link
// (NetworkManager routes over Ethernet first when both are up).
func parseVia(out string) string {
	via := ""
	for _, line := range strings.Split(out, "\n") {
		f := splitTerse(strings.TrimSpace(line))
		if len(f) < 2 || f[1] != "connected" {
			continue
		}
		switch f[0] {
		case "ethernet":
			return "ethernet"
		case "wifi":
			via = "wifi"
		}
	}
	return via
}

// NetworkManager answers "what is NM doing right now", cached briefly since the boot page polls.
type NetworkManager struct {
	run runner

	mu      sync.Mutex
	checked time.Time
	last    NMState
}

func (m *NetworkManager) State(ctx context.Context) NMState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.checked) < time.Second {
		return m.last
	}
	st := NMState{State: "unknown"}
	if out, err := m.run(ctx, "nmcli", "-t", "-f", "STATE", "general"); err == nil {
		st.State = parseGeneralState(string(out))
	}
	if out, err := m.run(ctx, "nmcli", "-t", "-f", "TYPE", "connection", "show"); err == nil {
		st.SavedWifi = countWifiProfiles(string(out))
	}
	if out, err := m.run(ctx, "nmcli", "-t", "-f", "TYPE,STATE", "device"); err == nil {
		st.Via = parseVia(string(out))
	}
	m.last, m.checked = st, time.Now()
	return st
}
