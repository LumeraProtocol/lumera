package cmd

import (
	"net"
	"strconv"
	"testing"
)

func TestLoopbackAddrForPublicIsStableAndUnique(t *testing.T) {
	t.Parallel()

	tests := []struct {
		public string
		want   string
	}{
		{public: "127.0.0.1:8545", want: "127.0.0.1:42336"},
		{public: "0.0.0.0:8645", want: "127.0.0.1:42436"},
		{public: "[::]:8745", want: "127.0.0.1:42536"},
		{public: "localhost:32768", want: "127.0.0.1:2047"},
		{public: "127.0.0.1:32863", want: "127.0.0.1:2142"},
	}

	seen := make(map[string]string, len(tests))
	for _, tc := range tests {
		t.Run(tc.public, func(t *testing.T) {
			got, err := loopbackAddrForPublic(tc.public)
			if err != nil {
				t.Fatalf("loopbackAddrForPublic(%q): %v", tc.public, err)
			}
			if got != tc.want {
				t.Fatalf("loopbackAddrForPublic(%q) = %q, want %q", tc.public, got, tc.want)
			}
			if previous, exists := seen[got]; exists {
				t.Fatalf("public addresses %q and %q mapped to the same internal address %q", previous, tc.public, got)
			}
			seen[got] = tc.public
		})
	}
}

func TestLoopbackAddrForPublicRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	for _, publicAddr := range []string{"", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:65536"} {
		if _, err := loopbackAddrForPublic(publicAddr); err == nil {
			t.Fatalf("loopbackAddrForPublic(%q) unexpectedly succeeded", publicAddr)
		}
	}
}

func TestReserveLoopbackAddrFallsBackWhenPrimaryIsOccupied(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on occupied primary: %v", err)
	}
	t.Cleanup(func() { _ = occupied.Close() })

	occupiedPort := occupied.Addr().(*net.TCPAddr).Port
	publicAddr := publicAddrForInternalPort(t, occupiedPort)
	got, err := reserveLoopbackAddr(publicAddr)
	if err != nil {
		t.Fatalf("reserveLoopbackAddr(%q): %v", publicAddr, err)
	}
	if got == occupied.Addr().String() {
		t.Fatalf("reserveLoopbackAddr(%q) returned occupied primary %q", publicAddr, got)
	}

	_, portText, err := net.SplitHostPort(got)
	if err != nil {
		t.Fatalf("parse fallback address %q: %v", got, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < firstUnprivilegedPort || port > lastUnprivilegedPort {
		t.Fatalf("fallback address %q is not unprivileged", got)
	}

	probe, err := net.Listen("tcp", got)
	if err != nil {
		t.Fatalf("fallback address %q is not available: %v", got, err)
	}
	_ = probe.Close()
}

func TestReserveLoopbackAddrSkipsLaterListenerPorts(t *testing.T) {
	const publicAddr = "0.0.0.0:39267" // primary candidate is default WS port 8546
	primary, err := loopbackAddrForPublic(publicAddr)
	if err != nil {
		t.Fatalf("loopbackAddrForPublic(%q): %v", publicAddr, err)
	}
	if primary != "127.0.0.1:8546" {
		t.Fatalf("test precondition: primary = %q, want default WS address", primary)
	}

	for _, excludedAddr := range []string{
		"127.0.0.1:8546",
		"tcp://127.0.0.1:8546",
	} {
		t.Run(excludedAddr, func(t *testing.T) {
			got, err := reserveLoopbackAddr(publicAddr, excludedAddr)
			if err != nil {
				t.Fatalf("reserveLoopbackAddr(%q): %v", publicAddr, err)
			}
			if got == "127.0.0.1:8546" {
				t.Fatalf("reserveLoopbackAddr(%q) selected excluded listener %q", publicAddr, got)
			}
		})
	}
}

func publicAddrForInternalPort(t *testing.T, internalPort int) string {
	t.Helper()

	want := net.JoinHostPort("127.0.0.1", strconv.Itoa(internalPort))
	for publicPort := firstUnprivilegedPort; publicPort <= lastUnprivilegedPort; publicPort++ {
		publicAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(publicPort))
		got, err := loopbackAddrForPublic(publicAddr)
		if err != nil {
			t.Fatalf("loopbackAddrForPublic(%q): %v", publicAddr, err)
		}
		if got == want {
			return publicAddr
		}
	}

	t.Fatalf("no public port maps to internal port %d", internalPort)
	return ""
}
