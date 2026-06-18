// Package logs streams EB instance logs for `uq logs <repo>`.
package logs

// Target 은 한 국가의 EB application 과 region.
type Target struct {
	Country string
	App     string
	Region  string
}

// Instance 는 EB 환경의 한 인스턴스.
type Instance struct {
	ID    string // EC2 인스턴스 id (범례/표시용)
	Num   int    // 1-base, eb ssh -n 번호
	Label string // 표시용, 예: "api-beta-kr-j21#1"
}

// Source 는 로그 소스 드라이버(eb/ecs/...) 추상화.
type Source interface {
	// Environments 는 target(app+region)의 Ready 환경명을 발견한다.
	Environments(t Target) ([]string, error)
	// Instances 는 환경의 인스턴스 목록을 반환한다(시작 시 1회 스냅샷).
	Instances(t Target, env string) ([]Instance, error)
	// TailArgs 는 한 인스턴스를 스트리밍하는 eb argv(eb 제외) 를 반환한다.
	// grep 이 비어있지 않으면 서버사이드로 필터한다(no-follow 는 파일 전체 검색).
	TailArgs(t Target, env string, inst Instance, follow bool, lines int, grep string) []string
}
