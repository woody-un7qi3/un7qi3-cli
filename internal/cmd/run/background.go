package run

import (
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

// runBackground spawns the profile's command(s) detached from the current
// shell, redirecting stdout+stderr to log files under ~/.cache/uq/run/<repo>/.
//
// Each child is given its own session (Setsid) so closing the parent terminal
// does not propagate SIGHUP, and so the user's `kill <pid>` stops just that
// process. Pid files are written next to the logs so the user can locate the
// process later.
func runBackground(w io.Writer, repo, dir string, p repocfg.Profile, env []string) error {
	logRoot, err := backgroundLogDir(repo)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logRoot, 0o755); err != nil {
		return fmt.Errorf("로그 디렉토리 생성 실패: %w", err)
	}

	var specs []bgSpec
	if len(p.Procs) > 0 {
		for _, pr := range p.Procs {
			cwd := dir
			if pr.Cwd != "" {
				cwd = filepath.Join(dir, pr.Cwd)
			}
			specs = append(specs, bgSpec{name: pr.Name, cwd: cwd, cmd: pr.Cmd, url: pr.URL})
		}
	} else {
		specs = append(specs, bgSpec{name: repo, cwd: dir, cmd: p.Cmd, url: p.URL})
	}

	fmt.Fprintln(w, output.Dim("백그라운드로 분리합니다…"))
	for _, s := range specs {
		if _, err := os.Stat(s.cwd); err != nil {
			return fmt.Errorf("[%s] cwd 가 없습니다: %s", s.name, s.cwd)
		}
		logPath := filepath.Join(logRoot, s.name+".log")
		pidPath := filepath.Join(logRoot, s.name+".pid")
		pid, err := spawnDetached(s.cwd, s.cmd, env, logPath)
		if err != nil {
			return fmt.Errorf("[%s] 시작 실패: %w", s.name, err)
		}
		_ = os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", pid)), 0o644)
		fmt.Fprintf(w, "  %s pid=%d  %s\n",
			output.Cyan("["+s.name+"]"),
			pid,
			output.Dim(fmt.Sprintf("로그: %s", tildify(logPath))),
		)
		if s.url != "" {
			fmt.Fprintf(w, "    %s %s\n", output.Dim("→"), output.Cyan(s.url))
		}
	}
	fmt.Fprintf(w, "\n%s %s\n",
		output.Dim("종료:"),
		output.Cyan(fmt.Sprintf("kill %s 또는 pkill -f '%s'", strings.Join(collectPids(specs, logRoot), " "), specs[0].cmd[0])),
	)
	return nil
}

func spawnDetached(dir string, cmd []string, env []string, logPath string) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("로그 파일 열기 실패 (%s): %w", logPath, err)
	}
	defer logFile.Close()

	c := osexec.Command(cmd[0], cmd[1:]...)
	c.Dir = dir
	c.Env = env
	c.Stdin = nil
	c.Stdout = logFile
	c.Stderr = logFile
	// Setsid: new session detached from parent's controlling terminal so SIGHUP
	// on shell exit doesn't kill the child.
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := c.Start(); err != nil {
		return 0, err
	}
	// Don't Wait — let it run independently. The release lets Go forget the child.
	pid := c.Process.Pid
	_ = c.Process.Release()
	return pid, nil
}

func backgroundLogDir(repo string) (string, error) {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "uq", "run", repo), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "uq", "run", repo), nil
}

type bgSpec struct {
	name string
	cwd  string
	cmd  []string
	url  string
}

func collectPids(specs []bgSpec, logRoot string) []string {
	pids := make([]string, 0, len(specs))
	for _, s := range specs {
		b, err := os.ReadFile(filepath.Join(logRoot, s.name+".pid"))
		if err != nil {
			continue
		}
		pids = append(pids, strings.TrimSpace(string(b)))
	}
	return pids
}

// tildify replaces a leading $HOME with "~" for terser display.
func tildify(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
