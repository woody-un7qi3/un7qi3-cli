package logs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// EBSource 는 eb ssh + aws elasticbeanstalk 로 동작하는 Source 구현.
type EBSource struct {
	path   string
	runner func(name string, args ...string) ([]byte, error)
}

// NewEBSource 는 기본 runner(uqexec.Run)를 쓰는 EB 드라이버를 만든다.
func NewEBSource(path string) *EBSource {
	return &EBSource{path: path, runner: uqexec.Run}
}

func (s *EBSource) Environments(t Target) ([]string, error) {
	out, err := s.runner("aws", "elasticbeanstalk", "describe-environments",
		"--application-name", t.App, "--region", t.Region,
		"--query", "Environments[?Status=='Ready'].EnvironmentName", "--output", "text")
	if err != nil {
		return nil, fmt.Errorf("환경 조회 실패(%s/%s): %w", t.App, t.Region, err)
	}
	return strings.Fields(string(out)), nil
}

func (s *EBSource) Instances(t Target, env string) ([]Instance, error) {
	out, err := s.runner("aws", "elasticbeanstalk", "describe-environment-resources",
		"--environment-name", env, "--region", t.Region, "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("인스턴스 조회 실패(%s): %w", env, err)
	}
	var parsed struct {
		EnvironmentResources struct {
			Instances []struct {
				Id string `json:"Id"`
			} `json:"Instances"`
		} `json:"EnvironmentResources"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("인스턴스 응답 파싱 실패: %w", err)
	}
	insts := make([]Instance, 0, len(parsed.EnvironmentResources.Instances))
	for i, in := range parsed.EnvironmentResources.Instances {
		insts = append(insts, Instance{
			ID:    in.Id,
			Num:   i + 1,
			Label: fmt.Sprintf("%s#%d", env, i+1),
		})
	}
	return insts, nil
}

func (s *EBSource) TailArgs(t Target, env string, inst Instance, follow bool, lines int) []string {
	tail := fmt.Sprintf("sudo tail -n %d", lines)
	if follow {
		tail += " -F"
	}
	tail += " " + s.path
	return []string{"ssh", env, "--region", t.Region, "-n", strconv.Itoa(inst.Num), "-c", tail}
}
