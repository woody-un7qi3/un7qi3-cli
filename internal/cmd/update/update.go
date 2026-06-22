// Package update implements the `uq update` command: GitHub Releases 기반
// 자기 업데이트. 비공개 레포여도 동작하도록 다운로드는 gh CLI 를 경유한다.
package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/version"
)

// releaseRepo 는 릴리즈가 발행되는 GitHub 레포(owner/repo).
//
// 회사 계정으로 이전할 때는 git origin 과 함께 이 한 줄만 바꾸면 된다
// (.goreleaser.yaml / 워크플로는 레포를 명시하지 않고 원격에서 추론한다).
const releaseRepo = "woody-un7qi3/un7qi3-cli"

// NewCmd returns the `uq upgrade` command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("uq 를 GitHub Releases 의 최신 버전으로 업그레이드합니다."),
		"",
		output.Desc("비공개 레포의 에셋을 받기 위해 ") + output.Cyan("gh") + output.Desc(" 인증을 사용합니다."),
		output.Desc("현재 실행 중인 바이너리를 제자리에서 교체합니다."),
	}, "\n")
	return &cobra.Command{
		Use:   "update",
		Short: "업데이트 확인 후 있으면 설치",
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd)
		},
	}
}

func runUpgrade(cmd *cobra.Command) error {
	w := cmd.OutOrStderr()

	// 1. gh 사전확인 (다운로드 인증 경로) — exit 4
	if !uqexec.LookPath("gh") {
		return &authpkg.RequiredError{Msg: "gh 미설치. `brew install gh && gh auth login` 실행"}
	}
	if s := authpkg.GhStatus(cmd.Context()); !s.OK {
		return &authpkg.RequiredError{Msg: "gh 인증 안 됨. `gh auth login` 실행"}
	}

	// 2. 최신 릴리즈 태그 + 릴리즈 노트(body) 조회 — 한 번의 호출로 함께 받는다.
	out, err := uqexec.Run("gh", "release", "view", "--repo", releaseRepo,
		"--json", "tagName,body")
	if err != nil {
		// 릴리즈가 하나도 없으면 gh 가 "release not found" 로 종료한다 — 에러가 아닌
		// 정상 상태로 안내한다.
		if strings.Contains(err.Error(), "release not found") {
			fmt.Fprintln(w, "발행된 릴리즈가 없습니다.")
			return nil
		}
		return fmt.Errorf("최신 릴리즈 조회 실패: %w", err)
	}
	var rel struct {
		TagName string `json:"tagName"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(out, &rel); err != nil {
		return fmt.Errorf("릴리즈 정보 파싱 실패: %w", err)
	}
	latest := strings.TrimSpace(rel.TagName)
	if latest == "" {
		fmt.Fprintln(w, "발행된 릴리즈가 없습니다.")
		return nil
	}
	if !needsUpgrade(version.Version, latest) {
		fmt.Fprintf(w, "이미 최신입니다 (%s).\n", latest)
		return nil
	}

	// 3. 현재 실행 파일 경로(심링크 해석) — 같은 디렉터리에서 원자적 교체
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("실행 파일 경로 확인 실패: %w", err)
	}
	if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
		exe = resolved
	}

	asset := assetName(runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(w, "%s → %s 업그레이드 중…\n", version.Version, latest)

	// 4. 같은 디렉터리에 임시 다운로드 후 rename (동일 파일시스템 → 원자적)
	tmp := exe + ".new"
	if _, err := uqexec.Run("gh", "release", "download", latest, "--repo", releaseRepo,
		"--pattern", asset, "--output", tmp, "--clobber"); err != nil {
		return fmt.Errorf("다운로드 실패(%s): %w", asset, err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("권한 설정 실패: %w", err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("교체 실패(%s): %w", exe, err)
	}

	fmt.Fprintf(w, "%s 업그레이드 완료: %s\n", output.Green("✓"), latest)
	if notes := formatReleaseNotes(rel.Body); notes != "" {
		fmt.Fprintf(w, "\n%s\n%s\n", output.Bold(latest+" 변경 내역:"), output.Desc(notes))
	}
	return nil
}

// reMarkdownLink 은 마크다운 링크 [텍스트](url) 에서 텍스트만 남긴다.
// reCommitRef 는 release-please 가 항목 끝에 붙이는 커밋 링크 ([해시](url)) 를 제거한다.
var (
	reMarkdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reCommitRef    = regexp.MustCompile(`\s*\(\[[0-9a-f]{7,40}\]\([^)]*\)\)\s*$`)
)

// formatReleaseNotes 는 release-please 의 마크다운 릴리즈 본문을 터미널에서 읽기
// 좋은 평문으로 바꾼다. 버전 제목(## …)은 호출부가 이미 출력하므로 버리고,
// 섹션 제목(### …)은 헤더 줄로, 항목(* / -)은 "· " 불릿으로 바꾼다. 끝에 붙는
// 커밋 해시 링크와 본문 안 마크다운 링크는 텍스트만 남기고 제거한다.
func formatReleaseNotes(body string) string {
	var out []string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "## "):
			continue
		case strings.HasPrefix(line, "###"):
			section := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, section)
		case strings.HasPrefix(line, "* "), strings.HasPrefix(line, "- "):
			item := strings.TrimSpace(line[1:])
			item = reCommitRef.ReplaceAllString(item, "")
			out = append(out, "· "+cleanInline(item))
		default:
			out = append(out, cleanInline(line))
		}
	}
	return strings.Join(out, "\n")
}

// cleanInline 은 한 줄에서 마크다운 인라인 장식을 걷어낸다: 링크 [텍스트](url) →
// 텍스트, 굵게 표시 **텍스트**/__텍스트__ 의 마커 제거.
func cleanInline(s string) string {
	s = reMarkdownLink.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	return s
}

// needsUpgrade 는 현재 버전과 최신 태그를 비교해 업그레이드가 필요한지 판단한다.
// 선행 'v' 와 공백을 무시한다. latest 가 비면 false(릴리즈 없음).
// dev 빌드는 어떤 릴리즈와도 달라 항상 업그레이드 대상이 된다.
func needsUpgrade(current, latest string) bool {
	l := normVer(latest)
	if l == "" {
		return false
	}
	return normVer(current) != l
}

// normVer 는 비교를 위해 공백과 선행 'v' 를 제거한다.
func normVer(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "v")
}

// assetName 은 GoReleaser 아카이브(formats: [binary], name_template:
// uq_{{ .Os }}_{{ .Arch }})가 만드는 에셋 이름과 일치하는 문자열을 만든다.
func assetName(goos, goarch string) string {
	return fmt.Sprintf("uq_%s_%s", goos, goarch)
}
