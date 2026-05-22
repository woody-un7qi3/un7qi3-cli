// Package doctor implements the `uq doctor` health check command.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// Check represents a single tool we probe.
type Check struct {
	Name     string
	Optional bool
	Fix      string
	Run      func() (ok bool, version string, detail string)
}

// Result is the JSON-friendly outcome of a check.
type Result struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Version  string `json:"version,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Fix      string `json:"fix,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

// Summary aggregates the results.
type Summary struct {
	OK       int `json:"ok"`
	Failed   int `json:"failed"`
	Optional int `json:"optional"`
}

// Report is the top-level JSON payload.
type Report struct {
	Checks  []Result `json:"checks"`
	Summary Summary  `json:"summary"`
}

// NewCmd returns the `uq doctor` command.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "필수 도구 설치 상태 점검",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			checks := buildChecks()
			results := make([]Result, 0, len(checks))
			var sum Summary

			for _, c := range checks {
				ok, ver, detail := c.Run()
				r := Result{
					Name:     c.Name,
					OK:       ok,
					Version:  ver,
					Detail:   detail,
					Optional: c.Optional,
				}
				if !ok {
					r.Fix = c.Fix
				}
				results = append(results, r)
				switch {
				case ok:
					sum.OK++
				case c.Optional:
					sum.Optional++
				default:
					sum.Failed++
				}
			}

			if jsonOut {
				if err := output.WriteJSON(cmd.OutOrStdout(), Report{Checks: results, Summary: sum}); err != nil {
					return err
				}
			} else {
				printHuman(cmd, results, sum)
			}

			if sum.Failed > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
}

func printHuman(cmd *cobra.Command, results []Result, sum Summary) {
	w := cmd.OutOrStdout()
	for _, r := range results {
		glyph := output.GlyphOK
		switch {
		case !r.OK && r.Optional:
			glyph = output.GlyphOptional
		case !r.OK:
			glyph = output.GlyphFail
		}
		right := r.Version
		if right == "" && r.Detail != "" {
			right = r.Detail
		}
		if !r.OK {
			if r.Optional {
				right = "점검 안 함 (선택)"
			} else if r.Detail != "" {
				right = r.Detail
			} else {
				right = "설치되지 않음"
			}
		} else if r.OK && r.Detail != "" {
			right = fmt.Sprintf("%s (%s)", r.Version, r.Detail)
		}
		fmt.Fprintf(w, "%s %-10s %s\n", glyph, r.Name, right)
		if !r.OK && !r.Optional && r.Fix != "" {
			fmt.Fprintf(w, "            → %s\n", r.Fix)
		}
	}
	fmt.Fprintln(w)
	if sum.Failed == 0 {
		fmt.Fprintf(w, "모든 점검 통과. (정상 %d개, 선택 %d개)\n", sum.OK, sum.Optional)
	} else {
		fmt.Fprintf(w, "%d개 문제 발견. 위의 권장 명령을 실행한 뒤 `uq doctor`를 다시 실행하세요.\n", sum.Failed)
	}
}

func buildChecks() []Check {
	return []Check{
		{
			Name: "git",
			Fix:  "xcode-select --install  # or: brew install git",
			Run:  versionCheck("git", []string{"--version"}, `git version (\S+)`),
		},
		{
			Name: "gh",
			Fix:  "brew install gh && gh auth login",
			Run:  ghCheck,
		},
		{
			Name: "node",
			Fix:  "brew install node  # or use nvm/fnm",
			Run:  versionCheck("node", []string{"--version"}, `v?(\S+)`),
		},
		{
			Name: "sdkman",
			Fix:  `curl -s "https://get.sdkman.io" | bash`,
			Run:  sdkmanCheck,
		},
		{
			Name: "java",
			Fix:  "sdk install java",
			Run:  javaCheck,
		},
		{
			Name: "aws",
			Fix:  "brew install awscli",
			Run:  versionCheck("aws", []string{"--version"}, `aws-cli/(\S+)`),
		},
		{
			Name: "gcloud",
			Fix:  "brew install --cask google-cloud-sdk",
			Run:  gcloudCheck,
		},
		{
			Name:     "docker",
			Optional: true,
			Fix:      "Docker Desktop 설치 (선택)",
			Run:      dockerCheck,
		},
	}
}

// versionCheck builds a check that runs `bin args...`, extracts the first
// regex group from stdout as the version, and reports ok=true on a successful
// exit. If the binary is missing we report ok=false.
func versionCheck(bin string, args []string, pattern string) func() (bool, string, string) {
	re := regexp.MustCompile(pattern)
	return func() (bool, string, string) {
		if _, err := exec.LookPath(bin); err != nil {
			return false, "", ""
		}
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			return false, "", strings.TrimSpace(string(out))
		}
		m := re.FindStringSubmatch(string(out))
		if len(m) >= 2 {
			return true, m[1], ""
		}
		return true, strings.TrimSpace(string(out)), ""
	}
}

func ghCheck() (bool, string, string) {
	if _, err := exec.LookPath("gh"); err != nil {
		return false, "", ""
	}
	out, err := exec.Command("gh", "--version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out))
	}
	re := regexp.MustCompile(`gh version (\S+)`)
	m := re.FindStringSubmatch(string(out))
	ver := ""
	if len(m) >= 2 {
		ver = m[1]
	}

	// Probe auth status — not fatal if unauthenticated, but report it.
	statusOut, statusErr := exec.Command("gh", "auth", "status").CombinedOutput()
	if statusErr != nil {
		return true, ver, "인증 안 됨"
	}
	userRe := regexp.MustCompile(`(?:Logged in to [^\s]+ (?:as|account) |account )(\S+)`)
	um := userRe.FindStringSubmatch(string(statusOut))
	if len(um) >= 2 {
		return true, ver, fmt.Sprintf("%s 로 인증됨", um[1])
	}
	return true, ver, "인증됨"
}

func sdkmanCheck() (bool, string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", err.Error()
	}
	initScript := home + "/.sdkman/bin/sdkman-init.sh"
	if _, err := os.Stat(initScript); err != nil {
		return false, "", "설치되지 않음"
	}
	verFile := home + "/.sdkman/var/version"
	if data, err := os.ReadFile(verFile); err == nil {
		return true, strings.TrimSpace(string(data)), ""
	}
	return true, "설치됨", ""
}

func javaCheck() (bool, string, string) {
	if _, err := exec.LookPath("java"); err != nil {
		return false, "", ""
	}
	out, err := exec.Command("java", "-version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out))
	}
	re := regexp.MustCompile(`version "([^"]+)"`)
	m := re.FindStringSubmatch(string(out))
	if len(m) >= 2 {
		return true, m[1], ""
	}
	return true, strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), ""
}

func gcloudCheck() (bool, string, string) {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return false, "", ""
	}
	out, err := exec.Command("gcloud", "--version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out))
	}
	re := regexp.MustCompile(`Google Cloud SDK (\S+)`)
	m := re.FindStringSubmatch(string(out))
	if len(m) >= 2 {
		return true, m[1], ""
	}
	return true, strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), ""
}

func dockerCheck() (bool, string, string) {
	if _, err := exec.LookPath("docker"); err != nil {
		return false, "", ""
	}
	out, err := exec.Command("docker", "--version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out))
	}
	re := regexp.MustCompile(`Docker version (\S+?),`)
	m := re.FindStringSubmatch(string(out))
	ver := ""
	if len(m) >= 2 {
		ver = strings.TrimSuffix(m[1], ",")
	} else {
		ver = strings.TrimSpace(string(out))
	}
	// Probe daemon
	if err := exec.Command("docker", "info").Run(); err != nil {
		return true, ver, "데몬 실행 안 됨"
	}
	return true, ver, ""
}
