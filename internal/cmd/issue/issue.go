// Package issue 는 `uq issue` — 터미널에서 이슈를 작성해 gh 로 올리는 명령을 제공한다.
package issue

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/project"
	"github.com/un7qi3inc/un7qi3-cli/internal/version"
)

// NewCmd 은 `uq issue` 명령을 만든다.
func NewCmd() *cobra.Command {
	var dryRun bool
	long := strings.Join([]string{
		output.Desc("터미널에서 기능 요청 / 버그 리포트 이슈를 작성해 GitHub 에 올립니다."),
		output.Desc("종류를 고르면 항목별 폼이 뜨고, 미리보기 확인 후 gh 로 제출합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq issue", "대화형으로 이슈 작성"),
		output.HelpExample("uq issue --dry-run", "제출 없이 조립된 본문만 확인"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "이슈 작성 (기능 요청 / 버그 리포트)",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssue(cmd, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "제출 없이 조립된 제목/라벨/본문만 출력")
	return cmd
}

func runIssue(cmd *cobra.Command, dryRun bool) error {
	w := cmd.OutOrStdout()

	// 1. 대화형 터미널 필수
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return clierr.PreconditionError{Msg: "uq issue 는 대화형 터미널이 필요합니다"}
	}

	// 2. gh 사전확인 (제출 인증 경로)
	if !uqexec.LookPath("gh") {
		return &authpkg.RequiredError{Msg: "gh 미설치. `brew install gh && gh auth login` 실행"}
	}
	if s := authpkg.GhStatus(cmd.Context()); !s.OK {
		return &authpkg.RequiredError{Msg: "gh 인증 안 됨. `gh auth login` 실행"}
	}

	// 3. 종류 선택
	f := form{kind: kindFeature}
	if err := huh.NewSelect[kind]().
		Title("이슈 종류").
		Options(
			huh.NewOption("기능 요청 (새 명령/개선)", kindFeature),
			huh.NewOption("버그 리포트", kindBug),
		).
		Value(&f.kind).
		Run(); err != nil {
		return err
	}

	// 4. 종류별 폼
	if err := runForm(&f); err != nil {
		return err
	}

	// 5. 미리보기
	fmt.Fprintln(w)
	fmt.Fprintln(w, output.Heading("미리보기"))
	fmt.Fprintf(w, "%s %s\n", output.Cyan("제목:"), f.title)
	fmt.Fprintf(w, "%s %s\n\n", output.Cyan("라벨:"), f.label())
	fmt.Fprintln(w, f.body())

	if dryRun {
		fmt.Fprintln(w, output.Dim("(--dry-run: 제출하지 않음)"))
		return nil
	}

	// 6. 확인 후 제출
	var confirm bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("%s 에 이슈를 올릴까요?", project.SelfRepo())).
		Value(&confirm).
		Run(); err != nil {
		return err
	}
	if !confirm {
		fmt.Fprintln(w, "취소했습니다.")
		return nil
	}

	url, err := createIssue(cmd.Context(), uqexec.Default(), project.SelfRepo(), f)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s %s\n", output.Green("이슈 생성됨:"), url)
	return nil
}

// runForm 은 종류에 맞는 huh 폼을 띄워 f 를 채운다. 제목은 공통 필수.
func runForm(f *form) error {
	var group *huh.Group
	titleField := huh.NewInput().Title("제목").Value(&f.title).
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("제목은 필수입니다")
			}
			return nil
		})

	if f.kind == kindBug {
		// 버그 리포트에만 쓰는 필드는 여기서 기본값을 채운다.
		if f.version == "" {
			f.version = version.Version
		}
		if f.env == "" {
			f.env = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
		}
		group = huh.NewGroup(
			titleField,
			huh.NewText().Title("무슨 일이 일어났나요?").Description("기대한 동작과 실제 동작").Value(&f.what),
			huh.NewText().Title("재현 방법").Value(&f.repro),
			huh.NewInput().Title("uq 버전").Value(&f.version),
			huh.NewInput().Title("환경").Description("OS/칩 (자동 채움, 비워도 됨)").Value(&f.env),
		)
	} else {
		group = huh.NewGroup(
			titleField,
			huh.NewText().Title("해결하려는 문제").Description("지금 무엇이 불편한가요?").Value(&f.problem),
			huh.NewText().Title("제안").Description("어떻게 동작하면 좋을까요? (예시 명령/옵션)").Value(&f.proposal),
			huh.NewText().Title("완료 기준 (선택)").Value(&f.acceptance),
		)
	}
	return huh.NewForm(group).Run()
}
