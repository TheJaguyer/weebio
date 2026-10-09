package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestSplitTerse(t *testing.T) {
	got := splitTerse(`*:My\:Net\\5G:72:WPA2`)
	want := []string{"*", `My:Net\5G`, "72", "WPA2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseWifiList(t *testing.T) {
	out := " :Home:40:WPA2\n" +
		"*:Home:65:WPA2\n" + // active entry wins even though another AP of the same SSID exists
		" :Cafe:80:\n" + // open network
		" ::90:WPA2\n" + // hidden network, skipped
		" :Cafe:30:\n" +
		" :Neighbour:55:WPA1 WPA2\n" +
		"\n"
	got := parseWifiList(out)
	want := []Network{
		{SSID: "Home", Signal: 65, Secure: true, Active: true},
		{SSID: "Cafe", Signal: 80, Secure: false},
		{SSID: "Neighbour", Signal: 55, Secure: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestConnectPassesPasswordOnlyWhenGiven(t *testing.T) {
	var calls [][]string
	w := &Wifi{run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return nil, nil
	}}
	_ = w.Connect(context.Background(), "Cafe", "")
	_ = w.Connect(context.Background(), "Home", "hunter2")
	if got := calls[0][len(calls[0])-1]; got != "Cafe" {
		t.Errorf("open network: last arg %q, want SSID", got)
	}
	if got := calls[1][len(calls[1])-2:]; !reflect.DeepEqual(got, []string{"password", "hunter2"}) {
		t.Errorf("secure network: tail %q", got)
	}
}

func TestConnectWrongPassword(t *testing.T) {
	w := &Wifi{run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("Error: Connection activation failed: Secrets were required, but not provided."), errors.New("exit status 4")
	}}
	if err := w.Connect(context.Background(), "Home", "nope"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("got %v, want ErrWrongPassword", err)
	}
}
