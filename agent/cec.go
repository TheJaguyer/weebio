package main

// HDMI-CEC follower: answers the TV the way a streaming stick does, so the TV forwards its remote's
// buttons to the box. Joining the bus (logical address, OSD name) and the button keymap are done at
// boot by weebio-cec; the kernel turns forwarded buttons into key presses. This adds the replies the
// kernel leaves to userspace: Active Source when the TV selects our input, Menu Status, power and deck
// status. The box never claims the TV by itself (that would switch or wake TVs on every reboot).

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// CEC opcodes used here (HDMI-CEC 1.4/2.0).
const (
	opFeatureAbort          = 0x00
	opDeckStatus            = 0x1b
	opGiveDeckStatus        = 0x1a
	opRoutingChange         = 0x80
	opRoutingInformation    = 0x81
	opActiveSource          = 0x82
	opRequestActiveSource   = 0x85
	opSetStreamPath         = 0x86
	opMenuRequest           = 0x8d
	opMenuStatus            = 0x8e
	opGiveDevicePowerStatus = 0x8f
	opReportPowerStatus     = 0x90

	addrTV        = 0x0
	addrBroadcast = 0xf

	abortUnrecognized = 0x00
	powerOn           = 0x00
	deckStop          = 0x1a
	menuActivated     = 0x00
)

// Messages the kernel's CEC core answers itself (or that are replies/notices); never feature-abort them.
var coreHandled = map[byte]bool{
	0x00: true, // Feature Abort
	0x44: true, // User Control Pressed  (turned into key presses by the kernel)
	0x45: true, // User Control Released
	0x46: true, // Give OSD Name
	0x47: true, // Set OSD Name
	0x83: true, // Give Physical Address
	0x84: true, // Report Physical Address
	0x87: true, // Device Vendor ID
	0x8c: true, // Give Device Vendor ID
	0x9e: true, // CEC Version
	0x9f: true, // Get CEC Version
	0xa5: true, // Give Features
	0xa6: true, // Report Features
	0xff: true, // Abort
}

// cecFollower holds our bus identity and whether we are the TV's active source.
type cecFollower struct {
	physAddr uint16 // e.g. 0x3000 for 3.0.0.0
	logAddr  byte   // e.g. 8 (Playback Device 2)
	active   bool
}

func cecMsg(from, to byte, payload ...byte) []byte {
	return append([]byte{from<<4 | to}, payload...)
}

func (f *cecFollower) becomeActive() [][]byte {
	f.active = true
	return [][]byte{
		cecMsg(f.logAddr, addrBroadcast, opActiveSource, byte(f.physAddr>>8), byte(f.physAddr)),
		cecMsg(f.logAddr, addrTV, opMenuStatus, menuActivated),
	}
}

// selected handles a "the TV now shows physical address pa" notice.
func (f *cecFollower) selected(pa uint16) [][]byte {
	if pa == f.physAddr {
		return f.becomeActive()
	}
	f.active = false
	return nil
}

// handle returns the replies to one received CEC message (header byte, opcode, operands).
func (f *cecFollower) handle(msg []byte) [][]byte {
	if len(msg) < 2 {
		return nil // polling message
	}
	from, to, op := msg[0]>>4, msg[0]&0x0f, msg[1]
	directed := to == f.logAddr
	pa := func(i int) uint16 { return uint16(msg[i])<<8 | uint16(msg[i+1]) }

	switch {
	case op == opSetStreamPath && len(msg) >= 4:
		return f.selected(pa(2))
	case op == opRoutingInformation && len(msg) >= 4:
		return f.selected(pa(2))
	case op == opRoutingChange && len(msg) >= 6:
		return f.selected(pa(4))
	case op == opActiveSource && len(msg) >= 4:
		if pa(2) != f.physAddr {
			f.active = false // another device took over
		}
		return nil
	case op == opRequestActiveSource:
		if f.active {
			return [][]byte{cecMsg(f.logAddr, addrBroadcast, opActiveSource, byte(f.physAddr>>8), byte(f.physAddr))}
		}
		return nil
	case !directed:
		return nil // other broadcasts, or messages for other devices
	case op == opMenuRequest:
		return [][]byte{cecMsg(f.logAddr, from, opMenuStatus, menuActivated)}
	case op == opGiveDevicePowerStatus:
		return [][]byte{cecMsg(f.logAddr, from, opReportPowerStatus, powerOn)}
	case op == opGiveDeckStatus:
		return [][]byte{cecMsg(f.logAddr, from, opDeckStatus, deckStop)}
	case coreHandled[op]:
		return nil
	default:
		// With a follower registered the kernel no longer rejects unknown messages; the spec says we must.
		return [][]byte{cecMsg(f.logAddr, from, opFeatureAbort, op, abortUnrecognized)}
	}
}

// ---- kernel interface (linux/cec.h; layouts measured on arm64) ----

const (
	cecAdapGPhysAddr = 0x80026101
	cecAdapGLogAddrs = 0x805c6103
	cecTransmit      = 0xc0386105
	cecReceive       = 0xc0386106
	cecSMode         = 0x40046109
	cecModeInitiator = 0x01
	cecModeFollower  = 0x10
)

type kernelCECMsg struct {
	TxTS, RxTS    uint64
	Len, Timeout  uint32
	Sequence      uint32
	Flags         uint32
	Msg           [16]byte
	Reply         byte
	RxStatus      byte
	TxStatus      byte
	TxArbLostCnt  byte
	TxNackCnt     byte
	TxLowDriveCnt byte
	TxErrorCnt    byte
	_             byte
}

type kernelLogAddrs struct {
	LogAddr     [4]byte
	LogAddrMask uint16
	CECVersion  byte
	NumLogAddrs byte
	_           [84]byte // vendor id, flags, osd name, device types, features
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// cecIdentity reads the adapter's current physical and (first) logical address; ok is false until
// weebio-cec has joined the bus or while the HDMI cable is unplugged.
func cecIdentity(fd uintptr) (pa uint16, la byte, ok bool) {
	var las kernelLogAddrs
	if ioctl(fd, cecAdapGPhysAddr, unsafe.Pointer(&pa)) != nil || pa == 0xffff {
		return 0, 0, false
	}
	if ioctl(fd, cecAdapGLogAddrs, unsafe.Pointer(&las)) != nil || las.NumLogAddrs == 0 || las.LogAddr[0] == 0xff {
		return 0, 0, false
	}
	return pa, las.LogAddr[0], true
}

func transmit(fd uintptr, msg []byte) error {
	var m kernelCECMsg
	m.Len = uint32(copy(m.Msg[:], msg))
	m.Timeout = 1000
	return ioctl(fd, cecTransmit, unsafe.Pointer(&m))
}

// followCEC serves one adapter until ctx is done, reopening it if it disappears.
func followCEC(ctx context.Context, path string) {
	f := &cecFollower{}
	for ctx.Err() == nil {
		if err := followCECOnce(ctx, path, f); err != nil {
			log.Printf("cec %s: %v (retrying)", path, err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

func followCECOnce(ctx context.Context, path string, f *cecFollower) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	fd := file.Fd()
	mode := uint32(cecModeInitiator | cecModeFollower)
	if err := ioctl(fd, cecSMode, unsafe.Pointer(&mode)); err != nil {
		return fmt.Errorf("follower mode: %w", err)
	}
	log.Printf("cec %s: following", path)
	for ctx.Err() == nil {
		var m kernelCECMsg
		m.Timeout = 1000 // ms; wake regularly so ctx cancellation is noticed
		if err := ioctl(fd, cecReceive, unsafe.Pointer(&m)); err != nil {
			if err == syscall.ETIMEDOUT || err == syscall.EINTR {
				continue
			}
			return fmt.Errorf("receive: %w", err)
		}
		pa, la, ok := cecIdentity(fd)
		if !ok || m.Len == 0 || m.Len > 16 {
			continue
		}
		f.physAddr, f.logAddr = pa, la
		for _, reply := range f.handle(m.Msg[:m.Len]) {
			if err := transmit(fd, reply); err != nil {
				log.Printf("cec %s: transmit % x: %v", path, reply, err)
			}
		}
	}
	return nil
}

// startCEC follows every HDMI-CEC adapter present (/dev/cec0 is HDMI 0, next to the power socket).
func startCEC(ctx context.Context) {
	paths, _ := filepath.Glob("/dev/cec*")
	for _, path := range paths {
		go followCEC(ctx, path)
	}
}
