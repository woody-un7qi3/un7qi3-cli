// Package project 는 uq 가 참조하는 GitHub 레포 정체성을 한곳에서 제공한다.
// 회사 계정 이전 시 git origin 과 함께 selfRepo/org 만 바꾸면 된다.
package project

import "os"

const (
	// selfRepo 는 릴리스가 발행되고 이슈가 등록되는 un7qi3-cli 레포(owner/name).
	selfRepo = "woody-un7qi3/un7qi3-cli"
	// org 는 `uq repo` 가 스캔하는 GitHub 조직.
	org = "un7qi3inc"
)

// SelfRepo 는 릴리스·이슈 대상 레포(owner/name)를 반환한다.
// UQ_REPO 환경변수가 있으면 그 값이 우선한다(테스트/개인 레포용).
func SelfRepo() string {
	if v := os.Getenv("UQ_REPO"); v != "" {
		return v
	}
	return selfRepo
}

// Org 는 조직 레포 스캔용 GitHub 조직명을 반환한다.
func Org() string { return org }
