package issue

import (
	"strings"
	"testing"
)

func TestLabel(t *testing.T) {
	if got := (form{kind: kindFeature}).label(); got != "enhancement" {
		t.Errorf("feature label = %q", got)
	}
	if got := (form{kind: kindBug}).label(); got != "bug" {
		t.Errorf("bug label = %q", got)
	}
}

func TestFeatureBody(t *testing.T) {
	f := form{
		kind:     kindFeature,
		problem:  "포트 충돌이 잦다",
		proposal: "uq run --port 로 지정",
	}
	b := f.body()
	for _, want := range []string{"## 해결하려는 문제", "포트 충돌이 잦다", "## 제안", "uq run --port 로 지정"} {
		if !strings.Contains(b, want) {
			t.Errorf("body missing %q\n--- body ---\n%s", want, b)
		}
	}
	if strings.Contains(b, "## 완료 기준") {
		t.Errorf("빈 완료 기준 섹션이 생략되지 않음:\n%s", b)
	}
}

func TestFeatureBodyWithAcceptance(t *testing.T) {
	f := form{kind: kindFeature, problem: "p", proposal: "q", acceptance: "테스트 통과"}
	if !strings.Contains(f.body(), "## 완료 기준") {
		t.Errorf("완료 기준 섹션 누락:\n%s", f.body())
	}
}

func TestBugBody(t *testing.T) {
	f := form{
		kind:    kindBug,
		what:    "패널이 안 뜸",
		repro:   "1. uq run\n2. ...",
		version: "0.1.8",
		env:     "darwin/arm64",
	}
	b := f.body()
	for _, want := range []string{"## 무슨 일이 일어났나요?", "패널이 안 뜸", "## 재현 방법", "1. uq run", "## uq 버전", "0.1.8", "## 환경", "darwin/arm64"} {
		if !strings.Contains(b, want) {
			t.Errorf("body missing %q\n--- body ---\n%s", want, b)
		}
	}
}

func TestBugBodyOmitsEmptyEnv(t *testing.T) {
	f := form{kind: kindBug, what: "w", repro: "r", version: "0.1.8"}
	if strings.Contains(f.body(), "## 환경") {
		t.Errorf("빈 환경 섹션이 생략되지 않음:\n%s", f.body())
	}
}
