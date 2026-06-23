package issue

import "strings"

// kind 는 작성할 이슈 종류.
type kind int

const (
	kindFeature kind = iota
	kindBug
)

// form 은 TUI 로 채운 이슈 입력값. 종류에 따라 일부 필드만 사용한다.
type form struct {
	kind  kind
	title string
	// 기능 요청
	problem    string
	proposal   string
	acceptance string
	// 버그 리포트
	what    string
	repro   string
	version string
	env     string
}

// label 은 GitHub 이슈 템플릿과 일치하는 라벨을 반환한다.
func (f form) label() string {
	if f.kind == kindBug {
		return "bug"
	}
	return "enhancement"
}

// body 는 GitHub 이슈 템플릿과 동일한 섹션 제목으로 마크다운 본문을 만든다.
// 빈 선택 필드는 섹션째 생략한다.
func (f form) body() string {
	var b strings.Builder
	section := func(title, content string) {
		if strings.TrimSpace(content) == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## ")
		b.WriteString(title)
		b.WriteString("\n")
		b.WriteString(strings.TrimRight(content, "\n"))
		b.WriteString("\n")
	}
	if f.kind == kindBug {
		section("무슨 일이 일어났나요?", f.what)
		section("재현 방법", f.repro)
		section("uq 버전", f.version)
		section("환경", f.env)
	} else {
		section("해결하려는 문제", f.problem)
		section("제안", f.proposal)
		section("완료 기준", f.acceptance)
	}
	return b.String()
}
