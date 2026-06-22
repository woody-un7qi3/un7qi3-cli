package upgrade

import "testing"

func TestNeedsUpgrade(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"dev", "v0.1.0", true},       // 로컬 dev 빌드 → 항상 업그레이드
		{"0.1.0", "v0.1.0", false},    // v 접두 차이 무시 → 동일
		{"v0.1.0", "v0.1.0", false},   // 동일
		{"0.1.0", "v0.2.0", true},     // 더 새 버전
		{"0.1.0", "", false},          // 릴리즈 정보 없음 → 업그레이드 안 함
		{" v0.1.0 ", "v0.1.0", false}, // 공백 무시
	}
	for _, c := range cases {
		if got := needsUpgrade(c.current, c.latest); got != c.want {
			t.Errorf("needsUpgrade(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestFormatReleaseNotes(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{
			name: "release-please 본문을 평문으로 렌더링",
			body: "## [0.1.2](https://x/compare/v0.1.1...v0.1.2) (2026-06-22)\n\n\n" +
				"### 리팩터\n\n" +
				"* 시니어 관점 전면 리팩토링 (에러계약·context) ([e04f12c](https://x/commit/e04f12c80))\n",
			want: "리팩터\n· 시니어 관점 전면 리팩토링 (에러계약·context)",
		},
		{
			name: "여러 섹션은 빈 줄로 구분",
			body: "## [1.0.0](url) (2026-01-01)\n\n" +
				"### 기능\n\n* 새 명령 추가 ([abc1234](url))\n\n" +
				"### 버그 수정\n\n* 충돌 수정 ([def5678](url))\n",
			want: "기능\n· 새 명령 추가\n\n버그 수정\n· 충돌 수정",
		},
		{
			name: "본문 안의 일반 링크는 텍스트만 남긴다",
			body: "### 문서\n\n* [README](https://x/README.md) 갱신 ([aaa0001](url))\n",
			want: "문서\n· README 갱신",
		},
		{
			name: "스코프 강조(**)는 제거한다",
			body: "### 기능\n\n* **upgrade:** 릴리즈 노트를 터미널 평문으로 렌더링 ([17b06a4](url))\n",
			want: "기능\n· upgrade: 릴리즈 노트를 터미널 평문으로 렌더링",
		},
		{
			name: "빈 본문은 빈 문자열",
			body: "",
			want: "",
		},
	}
	for _, c := range cases {
		if got := formatReleaseNotes(c.body); got != c.want {
			t.Errorf("%s\nformatReleaseNotes() =\n%q\nwant\n%q", c.name, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "uq_darwin_arm64"},
		{"darwin", "amd64", "uq_darwin_amd64"},
		{"linux", "amd64", "uq_linux_amd64"},
	}
	for _, c := range cases {
		if got := assetName(c.goos, c.goarch); got != c.want {
			t.Errorf("assetName(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}
