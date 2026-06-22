package upgrade

import "testing"

func TestNeedsUpgrade(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"dev", "v0.1.0", true},      // 로컬 dev 빌드 → 항상 업그레이드
		{"0.1.0", "v0.1.0", false},   // v 접두 차이 무시 → 동일
		{"v0.1.0", "v0.1.0", false},  // 동일
		{"0.1.0", "v0.2.0", true},    // 더 새 버전
		{"0.1.0", "", false},         // 릴리즈 정보 없음 → 업그레이드 안 함
		{" v0.1.0 ", "v0.1.0", false}, // 공백 무시
	}
	for _, c := range cases {
		if got := needsUpgrade(c.current, c.latest); got != c.want {
			t.Errorf("needsUpgrade(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
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
