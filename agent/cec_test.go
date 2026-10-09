package main

import (
	"reflect"
	"testing"
	"unsafe"
)

func TestKernelStructSizes(t *testing.T) {
	// Must match linux/cec.h exactly (measured with sizeof on arm64), or the ioctls fail.
	if s := unsafe.Sizeof(kernelCECMsg{}); s != 56 {
		t.Fatalf("cec_msg size %d, want 56", s)
	}
	if o := unsafe.Offsetof(kernelCECMsg{}.Msg); o != 32 {
		t.Fatalf("cec_msg.msg offset %d, want 32", o)
	}
	if s := unsafe.Sizeof(kernelLogAddrs{}); s != 92 {
		t.Fatalf("cec_log_addrs size %d, want 92", s)
	}
}

// The box is Playback Device 2 (logical 8) at physical 3.0.0.0, as on the test TV.
func newFollower() *cecFollower { return &cecFollower{physAddr: 0x3000, logAddr: 8} }

func TestSelectingOurInputMakesUsActive(t *testing.T) {
	f := newFollower()
	// TV (0) broadcasts Set Stream Path 3.0.0.0: someone picked the Pi's input.
	got := f.handle([]byte{0x0f, 0x86, 0x30, 0x00})
	want := [][]byte{
		{0x8f, 0x82, 0x30, 0x00}, // Active Source 3.0.0.0, broadcast
		{0x80, 0x8e, 0x00},       // Menu Status: activated, to the TV
	}
	if !reflect.DeepEqual(got, want) || !f.active {
		t.Fatalf("got % x (active=%v), want % x", got, f.active, want)
	}
	// Later the TV asks who is active: we answer.
	if got := f.handle([]byte{0x0f, 0x85}); !reflect.DeepEqual(got, [][]byte{{0x8f, 0x82, 0x30, 0x00}}) {
		t.Fatalf("request active source: % x", got)
	}
}

func TestOtherInputSelectedMeansNotActive(t *testing.T) {
	f := newFollower()
	f.handle([]byte{0x0f, 0x86, 0x30, 0x00})
	// Routing Change from 3.0.0.0 to 4.0.0.0 (the Fire TV Stick): no replies, and no longer active.
	if got := f.handle([]byte{0x0f, 0x80, 0x30, 0x00, 0x40, 0x00}); got != nil || f.active {
		t.Fatalf("got % x active=%v", got, f.active)
	}
	if got := f.handle([]byte{0x0f, 0x85}); got != nil {
		t.Fatalf("inactive box answered Request Active Source: % x", got)
	}
	// Another device announcing itself also ends our turn.
	f.handle([]byte{0x0f, 0x86, 0x30, 0x00})
	f.handle([]byte{0x4f, 0x82, 0x40, 0x00})
	if f.active {
		t.Fatal("still active after the Fire TV Stick became active source")
	}
}

func TestDirectedQueries(t *testing.T) {
	f := newFollower()
	cases := map[string]struct {
		in   []byte
		want [][]byte
	}{
		"menu request":      {[]byte{0x08, 0x8d, 0x02}, [][]byte{{0x80, 0x8e, 0x00}}},
		"power status":      {[]byte{0x08, 0x8f}, [][]byte{{0x80, 0x90, 0x00}}},
		"deck status":       {[]byte{0x08, 0x1a, 0x01}, [][]byte{{0x80, 0x1b, 0x1a}}},
		"samsung vendor":    {[]byte{0x08, 0xa0, 0x00, 0x00, 0xf0, 0x23}, [][]byte{{0x80, 0x00, 0xa0, 0x00}}},
		"remote key (core)": {[]byte{0x08, 0x44, 0x00}, nil},
		"osd name (core)":   {[]byte{0x08, 0x46}, nil},
		"not for us":        {[]byte{0x04, 0x8f}, nil},
		"polling":           {[]byte{0x08}, nil},
		"feature abort":     {[]byte{0x08, 0x00, 0x8f, 0x00}, nil},
	}
	for name, c := range cases {
		if got := f.handle(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got % x, want % x", name, got, c.want)
		}
	}
}

func TestNeverClaimsTheTVUnprompted(t *testing.T) {
	f := newFollower()
	if got := f.handle([]byte{0x0f, 0x85}); got != nil {
		t.Fatalf("answered Request Active Source without being selected: % x", got)
	}
}
