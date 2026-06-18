package run

import (
	"net"
	"testing"
)

func TestInspectPorts(t *testing.T) {
	// A live listener occupies a real port; closing a second listener frees its
	// port so we can assert the negative case too.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port

	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	freePort := free.Addr().(*net.TCPAddr).Port
	free.Close() // release it — now (almost certainly) nobody is listening

	statuses := InspectPorts([]int{busy, freePort})
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}
	if statuses[0].Port != busy || !statuses[0].InUse {
		t.Errorf("busy port %d: got %+v, want InUse", busy, statuses[0])
	}
	if statuses[1].Port != freePort || statuses[1].InUse {
		t.Errorf("free port %d: got %+v, want not InUse", freePort, statuses[1])
	}
}
