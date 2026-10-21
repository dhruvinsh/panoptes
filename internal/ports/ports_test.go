package ports

import (
	"bytes"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func P(dev string, vid, pid int, desc string) Port {
	if desc == "" {
		desc = "n/a"
	}
	return Port{Device: dev, VID: vid, PID: pid, Description: desc, HWID: "n/a"}
}

func devices(ps []Port) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Device)
	}
	return out
}

func TestPrefersCH340OverOthers(t *testing.T) {
	lst := []Port{P("/dev/ttyUSB1", 0x10C4, 0xEA60, ""), P("/dev/ttyACM0", 0x2341, 0x0043, ""),
		P("/dev/ttyUSB0", 0x1A86, 0x7523, ""), P("/dev/ttyUSB2", 0x1A86, 0x55D4, "")}
	got := devices(Rank(lst, "linux"))
	want := []string{"/dev/ttyUSB0", "/dev/ttyUSB2", "/dev/ttyUSB1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestUnknownPortsExcluded(t *testing.T) {
	lst := []Port{P("/dev/ttyS0", 0, 0, ""), P("/dev/ttyACM0", 0x2341, 0x0043, "Arduino Uno")}
	if got := Rank(lst, "linux"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestDescriptionFallback(t *testing.T) {
	lst := []Port{P("COM7", 0, 0, "USB-SERIAL CH340 (COM7)"), P("COM3", 0, 0, "Communications Port")}
	if got := devices(Rank(lst, "windows")); !reflect.DeepEqual(got, []string{"COM7"}) {
		t.Fatalf("got %v", got)
	}
}

func TestMacOSPrefersCu(t *testing.T) {
	lst := []Port{P("/dev/tty.wchusbserial1410", 0x1A86, 0x7523, ""), P("/dev/cu.wchusbserial1410", 0x1A86, 0x7523, "")}
	if got := Rank(lst, "darwin")[0].Device; got != "/dev/cu.wchusbserial1410" {
		t.Fatalf("got %s", got)
	}
	// darwin without VID/PID (ioreg unavailable): name hints still match
	lst = []Port{P("/dev/cu.Bluetooth-Incoming-Port", 0, 0, ""), P("/dev/cu.SLAB_USBtoUART", 0, 0, ""),
		P("/dev/cu.usbserial-1410", 0, 0, "")}
	if got := devices(Rank(lst, "darwin")); !reflect.DeepEqual(got, []string{"/dev/cu.SLAB_USBtoUART", "/dev/cu.usbserial-1410"}) {
		t.Fatalf("got %v", got)
	}
}

func TestStableOrderForTies(t *testing.T) {
	lst := []Port{P("/dev/ttyUSB3", 0x1A86, 0x7523, ""), P("/dev/ttyUSB1", 0x1A86, 0x7523, "")}
	if got := devices(Rank(lst, "linux")); !reflect.DeepEqual(got, []string{"/dev/ttyUSB3", "/dev/ttyUSB1"}) {
		t.Fatalf("got %v", got)
	}
}

func withFakes(t *testing.T, lst []Port, existing map[string]string) {
	t.Helper()
	oldL, oldE, oldR := listPorts, exists, realpath
	listPorts = func() ([]Port, error) { return lst, nil }
	exists = func(p string) bool { _, ok := existing[p]; return ok }
	realpath = func(p string) string {
		if r, ok := existing[p]; ok {
			return r
		}
		return p
	}
	t.Cleanup(func() { listPorts, exists, realpath = oldL, oldE, oldR })
}

func TestCandidatesWithFakeList(t *testing.T) {
	lst := []Port{P("/dev/ttyACM0", 0x2341, 0x0043, ""), P("/dev/ttyUSB0", 0x1A86, 0x7523, ""),
		P("/dev/ttyUSB1", 0x10C4, 0xEA60, "")}
	withFakes(t, lst, nil)
	if got := Candidates(); !reflect.DeepEqual(got, []string{"/dev/ttyUSB0", "/dev/ttyUSB1"}) {
		t.Fatalf("got %v", got)
	}
}

func TestSymlinkPreferred(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("udev symlinks are Linux only")
	}
	lst := []Port{P("/dev/ttyUSB0", 0x1A86, 0x7523, ""), P("/dev/ttyUSB1", 0x10C4, 0xEA60, "")}
	// both links exist: /dev/panoptes wins, the device it points to is not repeated
	withFakes(t, lst, map[string]string{"/dev/panoptes": "/dev/ttyUSB1", "/dev/cyd": "/dev/ttyUSB1"})
	if got := Candidates(); !reflect.DeepEqual(got, []string{"/dev/panoptes", "/dev/ttyUSB0"}) {
		t.Fatalf("got %v", got)
	}
	// only the legacy link
	withFakes(t, lst, map[string]string{"/dev/cyd": "/dev/ttyUSB0"})
	if got := Candidates(); !reflect.DeepEqual(got, []string{"/dev/cyd", "/dev/ttyUSB1"}) {
		t.Fatalf("got %v", got)
	}
}

func TestPrintList(t *testing.T) {
	withFakes(t, []Port{P("/dev/ttyS0", 0, 0, ""), P("/dev/ttyUSB0", 0x1A86, 0x7523, "USB Serial")}, nil)
	var b bytes.Buffer
	if PrintList(&b) != 0 {
		t.Fatal("exit code")
	}
	out := b.String()
	if !strings.Contains(out, "* /dev/ttyUSB0  1a86:7523  CH340  USB Serial") || !strings.Contains(out, "/dev/ttyS0") {
		t.Fatalf("got:\n%s", out)
	}
}

const ioregSample = `+-o AppleUSB20HubPort@14100000  <class IOUSBHostDevice, id 0x100000a01, registered, matched, active, busy 0 (5 ms), retain 20>
  | {
  |   "idProduct" = 2337
  |   "idVendor" = 1452
  |   "USB Product Name" = "USB2.0 Hub"
  | }
  |
  +-o USB Serial@14110000  <class IOUSBHostDevice, id 0x100000b02, registered, matched, active, busy 0 (13 ms), retain 29>
    | {
    |   "sessionID" = 123456789
    |   "idProduct" = 29987
    |   "USB Product Name" = "USB Serial"
    |   "idVendor" = 6790
    |   "USB Serial Number" = ""
    | }
    |
    +-o IOUSBHostInterface@0  <class IOUSBHostInterface, id 0x100000b05, registered, matched, active, busy 0 (2 ms), retain 7>
      | {
      |   "idProduct" = 29987
      |   "idVendor" = 6790
      | }
      |
      +-o AppleUSBCHCOM  <class AppleUSBCHCOM, id 0x100000b09, registered, matched, active, busy 0 (0 ms), retain 6>
        | {
        | }
        |
        +-o IOSerialBSDClient  <class IOSerialBSDClient, id 0x100000b0c, registered, matched, active, busy 0 (0 ms), retain 6>
            {
              "IOCalloutDevice" = "/dev/cu.usbserial-14110"
              "IODialinDevice" = "/dev/tty.usbserial-14110"
            }

+-o CP2102 USB to UART Bridge Controller@14200000  <class IOUSBHostDevice, id 0x100000c01, registered, matched, active, busy 0 (9 ms), retain 25>
  | {
  |   "idProduct" = 60000
  |   "idVendor" = 4292
  |   "USB Product Name" = "CP2102 USB to UART Bridge Controller"
  |   "USB Serial Number" = "0001"
  | }
  |
  +-o IOUSBHostInterface@0  <class IOUSBHostInterface, id 0x100000c05, registered, matched, active, busy 0 (2 ms), retain 7>
    +-o IOSerialBSDClient  <class IOSerialBSDClient, id 0x100000c0c, registered, matched, active, busy 0 (0 ms), retain 6>
        {
          "IOCalloutDevice" = "/dev/cu.SLAB_USBtoUART"
        }
`

func TestParseIoreg(t *testing.T) {
	m := ParseIoreg(ioregSample)
	ch := m["/dev/cu.usbserial-14110"]
	if ch.VID != 0x1A86 || ch.PID != 0x7523 || ch.Description != "USB Serial" {
		t.Fatalf("CH340: %+v", ch)
	}
	cp := m["/dev/cu.SLAB_USBtoUART"]
	if cp.VID != 0x10C4 || cp.PID != 0xEA60 || cp.SerialNumber != "0001" {
		t.Fatalf("CP2102: %+v", cp)
	}
	if len(m) != 2 {
		t.Fatalf("got %d entries: %v", len(m), m)
	}
	ranked := Rank([]Port{cp, ch}, "darwin")
	if ranked[0].Device != "/dev/cu.usbserial-14110" || Label(ranked[0]) != "CH340" {
		t.Fatalf("rank: %v", ranked)
	}
}

func TestPreferUSBIDs(t *testing.T) {
	saved := KnownUSBIDs
	defer func() { KnownUSBIDs = saved }()
	bad := PreferUSBIDs([]string{"0403:6001", "nope", "1a86:7523", "12345:1", " 0403:6001"})
	if len(bad) != 2 || bad[0] != "nope" || bad[1] != "12345:1" {
		t.Fatalf("bad = %v", bad)
	}
	if len(KnownUSBIDs) != len(saved)+1 || KnownUSBIDs[0].VID != 0x0403 || KnownUSBIDs[0].PID != 0x6001 {
		t.Fatalf("configured IDs go first, without duplicates: %v", KnownUSBIDs)
	}
	ranked := Rank([]Port{{Device: "a", VID: 0x1A86, PID: 0x7523}, {Device: "b", VID: 0x0403, PID: 0x6001}}, "linux")
	if len(ranked) != 2 || ranked[0].Device != "b" {
		t.Fatalf("configured bridge should rank first: %v", ranked)
	}
}
