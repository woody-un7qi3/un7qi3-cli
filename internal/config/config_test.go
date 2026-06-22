package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// expandPath 는 ~ 와 $VAR 를 절대 경로로 푼다. 존재 여부는 검증하지 않으며
// (최초 clone 전에는 디렉토리가 없는 게 정상), 확장 후 빈 경로가 되는 깨진
// 설정만 에러로 막는다.
func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	t.Run("absolute-passthrough", func(t *testing.T) {
		got, err := expandPath("/var/tmp/repos")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "/var/tmp/repos" {
			t.Errorf("got %q, want /var/tmp/repos", got)
		}
	})

	t.Run("tilde-only", func(t *testing.T) {
		got, err := expandPath("~")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != home {
			t.Errorf("got %q, want %q", got, home)
		}
	})

	t.Run("tilde-prefix", func(t *testing.T) {
		got, err := expandPath("~/un7qi3")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if want := filepath.Join(home, "un7qi3"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("env-var-expansion", func(t *testing.T) {
		t.Setenv("UQ_TEST_BASE", "/srv/code")
		got, err := expandPath("$UQ_TEST_BASE/repos")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "/srv/code/repos" {
			t.Errorf("got %q, want /srv/code/repos", got)
		}
	})

	t.Run("nonexistent-path-is-not-an-error", func(t *testing.T) {
		// 아직 만들어지지 않은 경로도 정상적으로 풀려야 한다 — 최초 clone 전 케이스.
		got, err := expandPath("/this/does/not/exist/yet")
		if err != nil {
			t.Fatalf("존재하지 않는 경로에 에러가 나면 안 됨: %v", err)
		}
		if got != "/this/does/not/exist/yet" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("empty-after-expansion-errors", func(t *testing.T) {
		// 정의되지 않은 변수 하나뿐이라 빈 문자열로 풀리는 깨진 설정.
		if _, err := expandPath("$UQ_DEFINITELY_UNSET_VAR_XYZ"); err == nil {
			t.Fatal("빈 경로로 확장되는 설정에 에러를 기대했지만 nil")
		}
	})

	t.Run("blank-input-errors", func(t *testing.T) {
		if _, err := expandPath("   "); err == nil {
			t.Fatal("공백 경로에 에러를 기대했지만 nil")
		}
	})
}

// ReposDir 의 우선순위(env > config > 기본값)와 확장이 함께 동작하는지 확인.
func TestReposDir_EnvOverride(t *testing.T) {
	t.Setenv(reposDirEnv, "/explicit/override")
	got, err := ReposDir()
	if err != nil {
		t.Fatalf("ReposDir: %v", err)
	}
	if got != "/explicit/override" {
		t.Errorf("got %q, want /explicit/override", got)
	}
}

func TestReposDir_DefaultWhenUnset(t *testing.T) {
	t.Setenv(reposDirEnv, "")
	// config 파일을 못 찾게(혹은 빈 설정이게) 격리된 XDG 경로를 가리킨다.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := ReposDir()
	if err != nil {
		t.Fatalf("ReposDir: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, defaultReposDirName)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, home) {
		t.Errorf("기본 경로는 HOME 하위여야 함: %q", got)
	}
}
