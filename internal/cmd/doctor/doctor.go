// Package doctor implements the `uq doctor` health check command.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	"github.com/un7qi3inc/un7qi3-cli/internal/config"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// Role names. A check with no Roles is "common" — shown to everyone.
const (
	RoleBackend  = "backend"
	RoleFrontend = "frontend"
	RoleInfra    = "infra"
)

// allRoles is the ordered set of selectable roles. Display order in the
// grouped output follows this slice, with "공통" rendered first.
var allRoles = []string{RoleBackend, RoleFrontend, RoleInfra}

// roleTitle maps internal role IDs to the heading shown to users.
var roleTitle = map[string]string{
	"":           "공통",
	RoleBackend:  "백엔드",
	RoleFrontend: "프런트",
	RoleInfra:    "인프라",
}

// Check represents a single tool we probe.
//
// Run returns ok=true when the tool is usable, plus the version, an optional
// human-friendly detail (e.g. "데몬 실행 안 됨"), and the resolved install
// path. Path is informational — usually the absolute binary location from
// exec.LookPath, or a tool-home dir for shell-script tools like sdkman.
//
// Roles assigns the tool to one or more team roles. An empty Roles slice
// means "common" — always shown regardless of --role filter. A tool can
// belong to multiple roles; in grouped display it is rendered under the
// first matching role (per allRoles ordering) to avoid duplicate listings.
type Check struct {
	Name     string
	Roles    []string
	Optional bool
	Fix      string
	// Usage, when set, returns a dynamic context string like
	// "used by: forceteller-app, forceteller-admin (2)" — rendered as a
	// sub-line under the tool's main row so users can decide whether they
	// actually need it. Called each time doctor runs; expected to be quick.
	Usage func() string
	Run   func() (ok bool, version, detail, path string)
}

// Result is the JSON-friendly outcome of a check.
type Result struct {
	Name     string   `json:"name"`
	OK       bool     `json:"ok"`
	Version  string   `json:"version,omitempty"`
	Detail   string   `json:"detail,omitempty"`
	Path     string   `json:"path,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	Usage    string   `json:"usage,omitempty"`
	Fix      string   `json:"fix,omitempty"`
	Optional bool     `json:"optional,omitempty"`
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
	long := strings.Join([]string{
		output.Desc("uq 사용에 필요한 외부 도구의 설치/인증 상태를 점검합니다."),
		"",
		output.Desc("점검 항목은 공통 / 백엔드 / 프런트 / 인프라 그룹으로 나뉩니다."),
		output.Yellow("--role") + output.Desc(" 로 본인 역할만 추려서 볼 수 있습니다 (공통은 항상 포함)."),
		output.Yellow("--json") + output.Desc(" 으로 머신 친화 출력. 하나라도 실패면 exit 1."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq doctor", "전체 그룹 점검"),
		output.HelpExample("uq doctor --role frontend", "공통 + 프런트만"),
		output.HelpExample("uq doctor --role backend,infra", "공통 + 백엔드 + 인프라"),
		output.HelpExample("uq doctor --json | jq '.summary'", "통과/실패 카운트"),
	}, "\n")
	var rolesFilter []string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "필수 도구 설치 상태 점검",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if err := validateRoles(rolesFilter); err != nil {
				return err
			}
			checks := filterByRoles(buildChecks(), rolesFilter)
			results := make([]Result, 0, len(checks))
			var sum Summary

			for _, c := range checks {
				ok, ver, detail, path := c.Run()
				r := Result{
					Name:     c.Name,
					OK:       ok,
					Version:  ver,
					Detail:   detail,
					Path:     path,
					Roles:    c.Roles,
					Optional: c.Optional,
				}
				if !ok {
					r.Fix = c.Fix
				}
				if c.Usage != nil {
					r.Usage = c.Usage()
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
				// 리포트 자체가 이미 사람/JSON 으로 출력됐다. 추가 메시지 없이
				// 실패를 알리는 런타임 에러만 반환하고, cobra 의 "Error: ..." 는
				// 막는다(exit code 는 main 의 Classify=1).
				cmd.SilenceErrors = true
				return clierr.PreconditionError{Msg: fmt.Sprintf("%d개 점검 실패", sum.Failed)}
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&rolesFilter, "role", nil, "팀 역할 필터 (backend, frontend, infra — 콤마 분리)")
	cmd.Flags().Bool("json", false, "JSON 형식으로 출력")
	return cmd
}

// validateRoles rejects unknown role names with a clear message.
func validateRoles(roles []string) error {
	if len(roles) == 0 {
		return nil
	}
	known := map[string]bool{RoleBackend: true, RoleFrontend: true, RoleInfra: true}
	for _, r := range roles {
		if !known[r] {
			return fmt.Errorf("알 수 없는 role '%s' — 사용 가능: backend, frontend, infra", r)
		}
	}
	return nil
}

// filterByRoles returns the subset of checks whose Roles intersect with
// filter. Common checks (empty Roles) are always kept. An empty filter
// keeps everything.
func filterByRoles(checks []Check, filter []string) []Check {
	if len(filter) == 0 {
		return checks
	}
	want := map[string]bool{}
	for _, r := range filter {
		want[r] = true
	}
	kept := make([]Check, 0, len(checks))
	for _, c := range checks {
		if len(c.Roles) == 0 {
			kept = append(kept, c)
			continue
		}
		for _, r := range c.Roles {
			if want[r] {
				kept = append(kept, c)
				break
			}
		}
	}
	return kept
}

// printHuman renders results grouped by role with section headers:
//
//	공통
//	GLYPH  name      version [detail]    /absolute/path
//	...
//	백엔드
//	...
//
// Each group's body is its own tab-aligned block (so columns line up within
// a group, but groups don't have to share padding — keeps lines tighter).
// A tool in multiple roles is rendered under the first matching role per
// allRoles ordering, avoiding duplicate listings.
func printHuman(cmd *cobra.Command, results []Result, sum Summary) {
	w := cmd.OutOrStdout()
	groups := groupResultsByRole(results)
	groupOrder := append([]string{""}, allRoles...) // 공통 먼저

	first := true
	for _, role := range groupOrder {
		rs := groups[role]
		if len(rs) == 0 {
			continue
		}
		if !first {
			fmt.Fprintln(w)
		}
		first = false
		fmt.Fprintln(w, output.Bold(roleTitle[role]))
		// Manual layout (no tabwriter) so we can place sub-lines indented
		// under the tool name rather than under the path column. We do a
		// two-pass: compute max widths of "  glyph name" and "version" across
		// the group, then emit each row with explicit padding.
		type rendered struct {
			head, version, tail string
			subLines            []string
		}
		rendered_rows := make([]rendered, 0, len(rs))
		nameW, verW := 0, 0
		for _, r := range rs {
			glyph := output.GlyphOK
			switch {
			case !r.OK && r.Optional:
				glyph = output.GlyphOptional
			case !r.OK:
				glyph = output.GlyphFail
			}
			var version, tail string
			// detailParts is everything we'll consider rendering as sub-lines.
			// Usage is appended last so it consistently shows at the bottom
			// of a tool's annotation block.
			var detailParts []string
			if r.OK {
				version = r.Version
				if r.Path != "" {
					tail = output.Blue(r.Path)
				}
				if r.Detail != "" {
					detailParts = append(detailParts, strings.Split(r.Detail, "\n")...)
				}
			} else {
				switch {
				case r.Optional:
					version = "점검 안 함 (선택)"
				case r.Detail != "":
					version = r.Detail
				default:
					version = "설치되지 않음"
				}
			}
			if r.Usage != "" {
				detailParts = append(detailParts, r.Usage)
			}
			// All description info (detail + usage) renders as a sub-line block
			// so the main row stays uniformly "glyph name version path" across
			// every tool — no inline detail, no width-dependent layout shift.
			var subLines []string
			if len(detailParts) > 0 {
				subLines = formatSubLines(detailParts)
			}
			head := "  " + glyph + " " + r.Name
			rendered_rows = append(rendered_rows, rendered{
				head: head, version: version, tail: tail, subLines: subLines,
			})
			if l := visibleLen(head); l > nameW {
				nameW = l
			}
			if l := visibleLen(version); l > verW {
				verW = l
			}
		}
		for _, rr := range rendered_rows {
			fmt.Fprintf(w, "%s%s  %s%s  %s\n",
				rr.head, strings.Repeat(" ", nameW-visibleLen(rr.head)),
				rr.version, strings.Repeat(" ", verW-visibleLen(rr.version)),
				rr.tail,
			)
			// Sub-lines indent to 4 spaces from start — directly under the
			// tool name (e.g. "    ├ global: ..." sits beneath "  ✓ git").
			for _, line := range rr.subLines {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}

	// Fix hints go under their own header so they aren't visually attached
	// to the last group (previously read as "belongs to 인프라").
	hintsPrinted := false
	for _, r := range results {
		if r.OK || r.Optional || r.Fix == "" {
			continue
		}
		if !hintsPrinted {
			fmt.Fprintln(w)
			fmt.Fprintln(w, output.Bold("해결 방법"))
			hintsPrinted = true
		}
		fmt.Fprintf(w, "  %s %s: %s\n", output.Dim("→"), r.Name, r.Fix)
	}
	fmt.Fprintln(w)
	if sum.Failed == 0 {
		fmt.Fprintf(w, "모든 점검 통과. (정상 %d개, 선택 %d개)\n", sum.OK, sum.Optional)
	} else {
		fmt.Fprintf(w, "%d개 문제 발견. 위의 권장 명령을 실행한 뒤 `uq doctor`를 다시 실행하세요.\n", sum.Failed)
	}
}

// visibleLen counts the terminal width of s.
//
// ASCII = 1 col per rune. Hangul (the only wide script we put in the table)
// = 2 cols per rune. Everything else is treated as 1 col — good enough for
// our cells, which are otherwise pure ASCII (versions, paths, tool names).
//
// Doctor's table cells never contain ANSI escape codes (those only appear
// in the dim/blue-wrapped tail column, which is rendered last and needs no
// alignment), so we don't strip escapes here.
func visibleLen(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r >= 0xAC00 && r <= 0xD7A3: // Hangul syllables
			n += 2
		case r >= 0x1100 && r <= 0x11FF: // Hangul Jamo
			n += 2
		default:
			n += 1
		}
	}
	return n
}

// formatSubLines turns a multi-line detail string into a visually aligned
// block rendered under a tool's main row. Each input line is treated as
// one of two shapes:
//
//	"<label>: <value>"   — a labeled attribute (e.g. "global: foo@bar.com",
//	                       "forceteller-admin: 17.0.0")
//	"⚠ <message>"        — a warning (no colon-split, full line preserved)
//	"<anything else>"    — a free-form annotation
//
// Output annotates each line with a tree connector (├ for inner, └ for the
// last) and pads labels so colons line up. The whole block is dim so it
// reads as secondary info under the main row.
//
// Heuristic for "labeled attribute": a colon counts as a label separator
// only when the part before it is a single token (no whitespace). This
// rejects prose like "1개가 global 이메일 사용 중: ..." while accepting
// long single-word labels like "forceteller-personalize-lambda:".
func formatSubLines(parts []string) []string {
	type parsed struct {
		prefix, label, value string
	}
	rows := make([]parsed, len(parts))
	for i, line := range parts {
		prefix := "├ "
		if i == len(parts)-1 {
			prefix = "└ "
		}
		row := parsed{prefix: prefix}
		switch {
		case strings.HasPrefix(line, "⚠"):
			row.value = line
		default:
			if idx := strings.IndexByte(line, ':'); idx > 0 {
				labelCandidate := line[:idx]
				if !strings.ContainsAny(labelCandidate, " \t") {
					row.label = line[:idx+1]
					row.value = strings.TrimSpace(line[idx+1:])
					break
				}
			}
			row.value = line
		}
		rows[i] = row
	}
	maxLabel := 0
	for _, r := range rows {
		if l := len(r.label); l > maxLabel {
			maxLabel = l
		}
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		var line string
		if r.label != "" {
			pad := strings.Repeat(" ", maxLabel-len(r.label)+1)
			line = r.prefix + r.label + pad + r.value
		} else {
			line = r.prefix + r.value
		}
		out[i] = output.Dim(line)
	}
	return out
}

// groupResultsByRole maps results into role buckets for grouped display.
// A result is bucketed into the first role it owns (per allRoles ordering)
// so tools with multiple roles aren't duplicated. Common results (empty
// Roles) go under "".
func groupResultsByRole(results []Result) map[string][]Result {
	groups := map[string][]Result{}
	roleOrder := allRoles
	for _, r := range results {
		if len(r.Roles) == 0 {
			groups[""] = append(groups[""], r)
			continue
		}
		// Pick the first role in allRoles ordering that this result owns.
		picked := ""
		for _, role := range roleOrder {
			for _, owned := range r.Roles {
				if owned == role {
					picked = role
					break
				}
			}
			if picked != "" {
				break
			}
		}
		if picked == "" {
			// Unknown role assigned — surface as common rather than dropping.
			groups[""] = append(groups[""], r)
		} else {
			groups[picked] = append(groups[picked], r)
		}
	}
	return groups
}

func buildChecks() []Check {
	return []Check{
		// 공통 — 누구나 필요한 베이스 toolchain.
		{
			Name: "git",
			Fix:  "xcode-select --install  # or: brew install git",
			Run:  gitCheck,
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

		// 백엔드 — JVM 생태계 + 컨테이너.
		{
			Name:  "sdkman",
			Roles: []string{RoleBackend},
			Fix:   `curl -s "https://get.sdkman.io" | bash`,
			Run:   sdkmanCheck,
		},
		{
			Name:  "java",
			Roles: []string{RoleBackend},
			Fix:   "sdk install java",
			Run:   javaCheck,
		},

		// 프런트 — forceteller-app (yarn 4.x), Angular CLI, Ionic CLI.
		// Usage 는 ~/un7qi3 의 어떤 레포가 그 도구를 실제로 쓰는지 동적으로 채움.
		{
			Name:  "yarn",
			Roles: []string{RoleFrontend},
			Fix:   "corepack enable && corepack prepare yarn@stable --activate",
			Usage: func() string { return un7qi3UsageSummary("yarn", "") },
			// yarn 은 거의 항상 글로벌 / corepack 으로 잡혀서 fallback 가치가
			// 낮지만, 누락 시 진단 일관성 위해 같은 패턴.
			Run: withLocalFallback(versionCheck("yarn", []string{"--version"}, `(\S+)`), "yarn"),
		},
		{
			Name:  "ng",
			Roles: []string{RoleFrontend},
			Fix:   "npm install -g @angular/cli",
			Usage: func() string { return un7qi3UsageSummary("ng", "@angular/cli") },
			Run:   ngCheck,
		},
		{
			Name:  "ionic",
			Roles: []string{RoleFrontend},
			Fix:   "npm install -g @ionic/cli",
			Usage: func() string { return un7qi3UsageSummary("ionic", "@ionic/cli") },
			Run:   withLocalFallback(versionCheck("ionic", []string{"--version"}, `(\S+)`), "ionic"),
		},

		// 인프라 — 클라우드 SDK.
		{
			Name:  "aws",
			Roles: []string{RoleInfra},
			Fix:   "brew install awscli",
			Run:   versionCheck("aws", []string{"--version"}, `aws-cli/(\S+)`),
		},
		{
			// eb 는 `uq logs` 전용 도구다. logs 를 안 쓰는 역할(프런트 등)에게
			// 강제하지 않도록 Optional 로 둔다 — 미설치여도 doctor 하드 실패가
			// 아니라 "선택"으로 표시. 실제 필요한 시점(uq logs)에 게이트가 잡는다.
			Name:     "eb",
			Roles:    []string{RoleInfra},
			Optional: true,
			Fix:      "brew install aws-elasticbeanstalk",
			Run:      versionCheck("eb", []string{"--version"}, `EB CLI (\S+)`),
		},
		{
			Name:  "gcloud",
			Roles: []string{RoleInfra},
			Fix:   "brew install --cask google-cloud-sdk",
			Run:   gcloudCheck,
		},
		{
			Name:     "docker",
			Roles:    []string{RoleInfra},
			Optional: true,
			Fix:      "Docker Desktop 설치 (선택)",
			Run:      dockerCheck,
		},
	}
}

// ngCheck looks for the Angular CLI. Global PATH first; if missing, it
// falls back to `~/un7qi3/<repo>/.../node_modules/.bin/ng` because the
// standard practice for Angular projects is per-repo install via devDeps —
// `npm run` puts node_modules/.bin on PATH automatically, so a missing
// global is not a real problem.
//
// On the local-fallback path we cannot reliably invoke `ng version`
// (Angular CLI errors out when invoked outside its workspace), so we read
// the version from the symlinked package's package.json instead.
func ngCheck() (bool, string, string, string) {
	if path, err := exec.LookPath("ng"); err == nil {
		out, err := exec.Command("ng", "version").CombinedOutput()
		if err != nil {
			return false, "", strings.TrimSpace(string(out)), path
		}
		re := regexp.MustCompile(`Angular CLI:\s*(\S+)`)
		m := re.FindStringSubmatch(string(out))
		if len(m) >= 2 {
			return true, m[1], "", path
		}
		return true, "설치됨", "", path
	}
	return localBinFallback("ng")
}

// withLocalFallback wraps a Run that probes the global PATH so that, on a
// failed global probe, it falls back to checking `~/un7qi3/.../node_modules/
// .bin/<binName>`. Use this for npm-distributed CLIs where local install is
// the norm (Angular CLI, Ionic CLI, etc.).
func withLocalFallback(run func() (bool, string, string, string), binName string) func() (bool, string, string, string) {
	return func() (bool, string, string, string) {
		ok, ver, detail, path := run()
		if ok {
			return ok, ver, detail, path
		}
		return localBinFallback(binName)
	}
}

// localBinFallback returns an "ok via local install" result when at least
// one ~/un7qi3 repo has the tool installed under its node_modules. The
// detail is a multi-line block — first a one-line note, then one line per
// repo with its specific version — so users can spot version drift across
// repos at a glance (e.g. one repo on Angular 9, another on Angular 17).
func localBinFallback(binName string) (bool, string, string, string) {
	ws := un7qi3WorkspaceDir()
	if ws == "" {
		return false, "", "", ""
	}
	locals := FindAllLocalBins(ws, binName)
	if len(locals) == 0 {
		return false, "", "", ""
	}
	lines := []string{"글로벌 미설치 (npm 스크립트 경유)"}
	for _, l := range locals {
		ver := l.Version
		if ver == "" {
			ver = "?"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", l.Repo, ver))
	}
	version := fmt.Sprintf("%d곳 로컬", len(locals))
	// path is intentionally empty — there is no single canonical install
	// to point at; the per-repo sub-lines carry that information.
	return true, version, strings.Join(lines, "\n"), ""
}

// versionCheck builds a check that runs `bin args...`, extracts the first
// regex group from stdout as the version, and reports ok=true on a successful
// exit. The path returned is exec.LookPath's resolution of bin (i.e. the
// absolute binary location actually on PATH). If the binary is missing we
// report ok=false.
func versionCheck(bin string, args []string, pattern string) func() (bool, string, string, string) {
	re := regexp.MustCompile(pattern)
	return func() (bool, string, string, string) {
		path, err := exec.LookPath(bin)
		if err != nil {
			return false, "", "", ""
		}
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			return false, "", strings.TrimSpace(string(out)), path
		}
		m := re.FindStringSubmatch(string(out))
		if len(m) >= 2 {
			return true, m[1], "", path
		}
		return true, strings.TrimSpace(string(out)), "", path
	}
}

// gitCheck reports the git version plus the global identity and, when a
// `~/un7qi3` workspace exists, a per-repo override summary.
//
// Why scan ~/un7qi3:
// Many users keep a personal email globally (woodeekim@gmail.com) and rely
// on per-repo local overrides (woody@un7qi3.co) for company work. A new
// clone without that local override would silently commit with the personal
// email. Surfacing the override pattern — and warning when a repo is
// missing it — turns a quiet footgun into a visible signal.
//
// Output schema (the first line goes in the description column; the rest
// are rendered as indented sub-lines aligned with that column):
//
//	global: <email> · <name>
//	~/un7qi3: <email1> × N  (repo, repo, ...)
//	~/un7qi3: <email2> × M  (repo, ...)
//	⚠ ~/un7qi3 레포 K개가 global 이메일 사용 중
func gitCheck() (bool, string, string, string) {
	path, err := exec.LookPath("git")
	if err != nil {
		return false, "", "", ""
	}
	out, err := exec.Command("git", "--version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out)), path
	}
	re := regexp.MustCompile(`git version (\S+)`)
	ver := ""
	if m := re.FindStringSubmatch(string(out)); len(m) >= 2 {
		ver = m[1]
	}

	globalEmail := strings.TrimSpace(stringOf(exec.Command("git", "config", "--global", "--get", "user.email").Output()))
	globalName := strings.TrimSpace(stringOf(exec.Command("git", "config", "--global", "--get", "user.name").Output()))

	var head string
	switch {
	case globalEmail == "" && globalName == "":
		head = "global: 미설정"
	case globalEmail == "":
		head = "global: " + globalName + " (email 미설정)"
	case globalName == "":
		head = "global: " + globalEmail + " (name 미설정)"
	default:
		head = "global: " + globalEmail + " · " + globalName
	}

	lines := []string{head}
	reposDir, _ := config.ReposDir()
	if reposDir != "" {
		overrides, usingGlobal := scanUn7qi3LocalEmails(reposDir, globalEmail)
		// Group by email so 같은 override 가 모이고 한 줄에 요약된다.
		byEmail := map[string][]string{}
		for repo, email := range overrides {
			byEmail[email] = append(byEmail[email], repo)
		}
		emails := make([]string, 0, len(byEmail))
		for e := range byEmail {
			emails = append(emails, e)
		}
		sort.Strings(emails)
		// 일치하는 override 는 카운트만 — 어떤 레포인지 알 필요 없고 줄만 길어진다.
		// 그룹이 여러 개면(여러 이메일 사용) 각각 별도 줄로.
		for _, e := range emails {
			lines = append(lines, fmt.Sprintf("~/un7qi3: %s × %d", e, len(byEmail[e])))
		}
		// 위험 케이스 (개인 메일로 commit 될 레포) 는 이름까지 다 보여줘야 행동 가능.
		if len(usingGlobal) > 0 {
			sort.Strings(usingGlobal)
			lines = append(lines, fmt.Sprintf("⚠ ~/un7qi3 레포 %d개가 global 이메일 사용 중: %s",
				len(usingGlobal), strings.Join(usingGlobal, ", ")))
		}
	}
	return true, ver, strings.Join(lines, "\n"), path
}

// scanUn7qi3LocalEmails walks workspaceDir's first-level subdirectories,
// treating each one that has a `.git/` as a repo and reading the
// **effective** `user.email` git would use when committing there.
//
// Effective (not --local) is the right scope because users routinely route
// company identity via `includeIf "gitdir:~/un7qi3/"` in ~/.gitconfig rather
// than per-repo local overrides. From doctor's perspective ("what email
// will my next commit use?") the includeIf result and a literal local
// override are equally good — both are non-personal-global identities.
//
// Per-repo `git config` calls run in parallel — 30 sequential exec()s on a
// cold filesystem cost ~1.5s; bounded concurrency cuts that to ~200ms.
//
// Repos whose effective email matches the bare global (i.e. no override at
// any layer) are reported in usingGlobal so the caller can warn. The rest
// are returned in overrides keyed by repo name.
func scanUn7qi3LocalEmails(workspaceDir, globalEmail string) (overrides map[string]string, usingGlobal []string) {
	overrides = map[string]string{}
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return overrides, nil
	}
	type result struct {
		name      string
		effective string
		ok        bool
	}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, 16) // 16 git processes at most
		results []result
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoDir := filepath.Join(workspaceDir, e.Name())
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
			continue
		}
		name := e.Name()
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out, err := exec.Command("git", "-C", repoDir, "config", "--get", "user.email").Output()
			mu.Lock()
			results = append(results, result{
				name:      name,
				effective: strings.TrimSpace(string(out)),
				ok:        err == nil,
			})
			mu.Unlock()
		}()
	}
	wg.Wait()

	for _, r := range results {
		if !r.ok || r.effective == "" || r.effective == globalEmail {
			usingGlobal = append(usingGlobal, r.name)
			continue
		}
		overrides[r.name] = r.effective
	}
	return overrides, usingGlobal
}

// stringOf swallows the error from cmd.Output() so we can chain trim — git
// config returns exit 1 when a key is unset, and we want "missing" reported
// as empty string, not as a hard failure.
func stringOf(b []byte, _ error) string {
	return string(b)
}

// ghCheck delegates the auth-status portion to internal/auth.GhStatus so that
// `uq doctor` and `uq auth status` share a single source of truth.
func ghCheck() (bool, string, string, string) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return false, "", "", ""
	}
	out, err := exec.Command("gh", "--version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out)), path
	}
	re := regexp.MustCompile(`gh version (\S+)`)
	m := re.FindStringSubmatch(string(out))
	ver := ""
	if len(m) >= 2 {
		ver = m[1]
	}

	// Delegate auth probe.
	s := authpkg.GhStatus()
	if !s.OK {
		return true, ver, "인증 안 됨", path
	}
	if s.User != "" {
		return true, ver, fmt.Sprintf("%s 로 인증됨", s.User), path
	}
	return true, ver, "인증됨", path
}

// sdkmanCheck probes ~/.sdkman because sdkman is a shell function, not a
// binary on PATH. The reported path is the sdkman home directory.
func sdkmanCheck() (bool, string, string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", err.Error(), ""
	}
	sdkHome := home + "/.sdkman"
	initScript := sdkHome + "/bin/sdkman-init.sh"
	if _, err := os.Stat(initScript); err != nil {
		return false, "", "설치되지 않음", ""
	}
	verFile := sdkHome + "/var/version"
	if data, err := os.ReadFile(verFile); err == nil {
		return true, strings.TrimSpace(string(data)), "", sdkHome
	}
	return true, "설치됨", "", sdkHome
}

func javaCheck() (bool, string, string, string) {
	path, err := exec.LookPath("java")
	if err != nil {
		return false, "", "", ""
	}
	out, err := exec.Command("java", "-version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out)), path
	}
	re := regexp.MustCompile(`version "([^"]+)"`)
	m := re.FindStringSubmatch(string(out))
	if len(m) >= 2 {
		return true, m[1], "", path
	}
	return true, strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), "", path
}

func gcloudCheck() (bool, string, string, string) {
	path, err := exec.LookPath("gcloud")
	if err != nil {
		return false, "", "", ""
	}
	out, err := exec.Command("gcloud", "--version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out)), path
	}
	re := regexp.MustCompile(`Google Cloud SDK (\S+)`)
	m := re.FindStringSubmatch(string(out))
	if len(m) >= 2 {
		return true, m[1], "", path
	}
	return true, strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), "", path
}

func dockerCheck() (bool, string, string, string) {
	path, err := exec.LookPath("docker")
	if err != nil {
		return false, "", "", ""
	}
	out, err := exec.Command("docker", "--version").CombinedOutput()
	if err != nil {
		return false, "", strings.TrimSpace(string(out)), path
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
		return true, ver, "데몬 실행 안 됨", path
	}
	return true, ver, "", path
}
