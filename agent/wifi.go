package main

import (
	"context"
	"errors"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Network is one Wi-Fi network as shown on the setup page.
type Network struct {
	SSID   string `json:"ssid"`
	Signal int    `json:"signal"` // 0-100
	Secure bool   `json:"secure"`
	Active bool   `json:"active"`
}

// runner executes a command and returns its combined output; swapped out in tests.
type runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// splitTerse splits one line of `nmcli -t` output, where ':' separates fields and
// literal ':' and '\' inside a value are escaped as '\:' and '\\'.
func splitTerse(line string) []string {
	var fields []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
		case c == ':':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(fields, cur.String())
}

// parseWifiList parses `nmcli -t -f IN-USE,SSID,SIGNAL,SECURITY dev wifi list`.
// Hidden networks are dropped and each SSID is kept once, preferring the active then strongest entry.
func parseWifiList(out string) []Network {
	best := map[string]Network{}
	for _, line := range strings.Split(out, "\n") {
		f := splitTerse(strings.TrimRight(line, "\r"))
		if len(f) < 4 || f[1] == "" {
			continue
		}
		signal, _ := strconv.Atoi(f[2])
		n := Network{
			SSID:   f[1],
			Signal: signal,
			Secure: f[3] != "" && f[3] != "--",
			Active: f[0] == "*",
		}
		if old, ok := best[n.SSID]; ok && (old.Active || (!n.Active && old.Signal >= n.Signal)) {
			continue
		}
		best[n.SSID] = n
	}
	networks := make([]Network, 0, len(best))
	for _, n := range best {
		networks = append(networks, n)
	}
	sort.Slice(networks, func(i, j int) bool {
		if networks[i].Active != networks[j].Active {
			return networks[i].Active
		}
		if networks[i].Signal != networks[j].Signal {
			return networks[i].Signal > networks[j].Signal
		}
		return networks[i].SSID < networks[j].SSID
	})
	return networks
}

type Wifi struct {
	run runner
}

func (w *Wifi) Networks(ctx context.Context, rescan bool) ([]Network, error) {
	mode := "auto"
	if rescan {
		mode = "yes"
	}
	out, err := w.run(ctx, "nmcli", "-t", "-f", "IN-USE,SSID,SIGNAL,SECURITY", "dev", "wifi", "list", "--rescan", mode)
	if err != nil {
		return nil, nmcliError(out, err)
	}
	return parseWifiList(string(out)), nil
}

// ErrWrongPassword is returned when NetworkManager rejects the credentials.
var ErrWrongPassword = errors.New("wrong password")

func (w *Wifi) Connect(ctx context.Context, ssid, password string) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	args := []string{"--wait", "40", "dev", "wifi", "connect", ssid}
	if password != "" {
		args = append(args, "password", password)
	}
	out, err := w.run(ctx, "nmcli", args...)
	if err != nil {
		return nmcliError(out, err)
	}
	return nil
}

// nmcliError turns nmcli's output into an error the setup page can explain.
func nmcliError(out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "secrets were required"),
		strings.Contains(lower, "802-1x supplicant"),
		strings.Contains(lower, "wrong password"):
		return ErrWrongPassword
	case msg != "":
		return errors.New(msg)
	default:
		return err
	}
}
