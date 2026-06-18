// Package run powers `uq run <repo>[:profile]`.
//
// port.go pre-flights the TCP ports a profile declares, so `uq run` can refuse
// to launch (and name the culprit) when a dev server's port is already taken,
// instead of letting the underlying tool die with a cryptic "address in use".
package run

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// PortStatus is the result of inspecting one TCP port on localhost.
type PortStatus struct {
	Port    int
	InUse   bool
	PID     int    // 0 if unknown
	Command string // short command name from lsof, "" if unknown
	Path    string // full command line of the owning process (ps), "" if unknown
	Cwd     string // working directory of the owning process (lsof), "" if unknown
}

// InspectPorts checks each port on 127.0.0.1, preserving input order.
//
// Whether a port is taken is decided by a short dial (reliable, no privileges).
// The owning PID/command is a best-effort lookup via lsof — absence of lsof, or
// any parse failure, just leaves PID/Command zero while InUse stays accurate.
func InspectPorts(ports []int) []PortStatus {
	out := make([]PortStatus, 0, len(ports))
	for _, p := range ports {
		s := PortStatus{Port: p}
		if portInUse(p) {
			s.InUse = true
			if pid, cmd, ok := portOwner(p); ok {
				s.PID, s.Command = pid, cmd
				s.Path = processCmdline(pid)
				s.Cwd = processCwd(pid)
			}
		}
		out = append(out, s)
	}
	return out
}

// portInUse reports whether something is accepting connections on the port.
func portInUse(port int) bool {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// portOwner asks lsof which process is listening on the port, returning its
// PID and command name. +c0 keeps the full command name (lsof truncates to 9
// chars by default).
func portOwner(port int) (pid int, command string, ok bool) {
	out, err := uqexec.Run("lsof", "-nP", "+c0", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN")
	if err != nil {
		return 0, "", false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		p, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		return p, fields[0], true
	}
	return 0, "", false
}

// processCmdline returns the full command line of pid via ps — the actual
// launched path + args (e.g. "node /Users/.../forceteller-admin/.../app.ts"),
// which tells you what is really holding the port. Empty if ps fails.
func processCmdline(pid int) string {
	out, err := uqexec.Run("ps", "-p", strconv.Itoa(pid), "-o", "command=")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// processCwd returns the working directory of pid via lsof's -Fn output (one
// `n<path>` line for the cwd descriptor). This is the most reliable "where is
// it running" signal when argv[0] is a bare name like "ng". Empty if unknown.
func processCwd(pid int) string {
	out, err := uqexec.Run("lsof", "-a", "-d", "cwd", "-p", strconv.Itoa(pid), "-Fn")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimPrefix(line, "n")
		}
	}
	return ""
}
