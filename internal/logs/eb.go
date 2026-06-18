package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// EBSource 는 eb ssh + aws elasticbeanstalk 로 동작하는 Source 구현.
type EBSource struct {
	path    string
	keyPath string // 비면 eb 기본 ssh. 설정되면 --custom 으로 accept-new 주입.
	runner  func(name string, args ...string) ([]byte, error)
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
	args := []string{"ssh", env, "--region", t.Region, "-n", strconv.Itoa(inst.Num)}
	if s.keyPath != "" {
		args = append(args, "--custom", sshCustom(s.keyPath))
	}
	return append(args, "-c", tail)
}

// sshCustom 은 eb ssh --custom 에 넘길 ssh 명령. StrictHostKeyChecking=accept-new 로
// 호스트 키 프롬프트를 없애 다중 인스턴스 동시 tail 시 stdin 충돌을 막고, 비대화형
// 접속을 보장한다. ConnectTimeout 으로 미허용 IP 의 행을 빨리 끊는다.
func sshCustom(keyPath string) string {
	return fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10", keyPath)
}

// ResolveKey 는 환경의 EC2 KeyName 을 조회해 ~/.ssh/<name>.pem 을 keyPath 로 설정한다.
// 키를 못 찾으면 keyPath 를 비워둬 eb 기본 ssh 로 폴백한다(이 경우 호스트 키 프롬프트 가능).
func (s *EBSource) ResolveKey(t Target, env string) error {
	out, err := s.runner("aws", "elasticbeanstalk", "describe-configuration-settings",
		"--application-name", t.App, "--environment-name", env, "--region", t.Region,
		"--query", "ConfigurationSettings[0].OptionSettings[?OptionName=='EC2KeyName'].Value",
		"--output", "text")
	if err != nil {
		return fmt.Errorf("EC2 KeyName 조회 실패(%s): %w", env, err)
	}
	name := strings.TrimSpace(string(out))
	if name == "" || name == "None" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
	}
	s.keyPath = filepath.Join(home, ".ssh", name+".pem")
	return nil
}
