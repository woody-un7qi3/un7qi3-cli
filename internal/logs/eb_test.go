package logs

import (
	"reflect"
	"strings"
	"testing"
)

func TestTailArgsWithKeyAddsCustom(t *testing.T) {
	s := NewEBSource("/var/log/web.stdout.log")
	s.keyPath = "/home/u/.ssh/k.pem"
	got := s.TailArgs(Target{Region: "ap-northeast-2"}, "api-prod-kr-j21", Instance{Num: 3}, true, 100)
	want := []string{"ssh", "api-prod-kr-j21", "--region", "ap-northeast-2", "-n", "3",
		"--custom", "ssh -i /home/u/.ssh/k.pem -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10",
		"-c", "sudo tail -n 100 -F /var/log/web.stdout.log"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("with-key argv\n got %v\nwant %v", got, want)
	}
}

func TestResolveKeySetsPemPath(t *testing.T) {
	s := NewEBSource("/p")
	s.runner = func(name string, args ...string) ([]byte, error) { return []byte("forceteller-service\n"), nil }
	if err := s.ResolveKey(Target{App: "a", Region: "r"}, "e"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(s.keyPath, "/.ssh/forceteller-service.pem") {
		t.Errorf("keyPath=%q", s.keyPath)
	}
}

func TestResolveKeyNoneLeavesEmpty(t *testing.T) {
	s := NewEBSource("/p")
	s.runner = func(name string, args ...string) ([]byte, error) { return []byte("None\n"), nil }
	if err := s.ResolveKey(Target{}, "e"); err != nil {
		t.Fatal(err)
	}
	if s.keyPath != "" {
		t.Errorf("keyPath 는 비어야 함, got %q", s.keyPath)
	}
}

func newFakeEB(out map[string][]byte) *EBSource {
	s := NewEBSource("/var/log/web.stdout.log")
	s.runner = func(name string, args ...string) ([]byte, error) {
		// 명령의 첫 서브커맨드로 응답을 고른다.
		key := args[0]
		return out[key], nil
	}
	return s
}

func TestTailArgsFollow(t *testing.T) {
	s := NewEBSource("/var/log/web.stdout.log")
	tgt := Target{Country: "kr", App: "kr-forceteller-api", Region: "ap-northeast-2"}
	got := s.TailArgs(tgt, "api-beta-kr-j21", Instance{Num: 2}, true, 100)
	want := []string{"ssh", "api-beta-kr-j21", "--region", "ap-northeast-2",
		"-n", "2", "-c", "sudo tail -n 100 -F /var/log/web.stdout.log"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("follow argv\n got %v\nwant %v", got, want)
	}
}

func TestTailArgsNoFollow(t *testing.T) {
	s := NewEBSource("/var/log/web.stdout.log")
	tgt := Target{Region: "ap-southeast-1"}
	got := s.TailArgs(tgt, "api-prod-en-j21", Instance{Num: 1}, false, 50)
	want := []string{"ssh", "api-prod-en-j21", "--region", "ap-southeast-1",
		"-n", "1", "-c", "sudo tail -n 50 /var/log/web.stdout.log"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no-follow argv\n got %v\nwant %v", got, want)
	}
}

func TestEnvironmentsParsesText(t *testing.T) {
	s := newFakeEB(map[string][]byte{
		"elasticbeanstalk": []byte("api-beta-kr-j21\tapi-prod-kr-j21\n"),
	})
	envs, err := s.Environments(Target{App: "kr-forceteller-api", Region: "ap-northeast-2"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api-beta-kr-j21", "api-prod-kr-j21"}
	if !reflect.DeepEqual(envs, want) {
		t.Errorf("envs got %v want %v", envs, want)
	}
}

func TestInstancesParsesJSON(t *testing.T) {
	s := newFakeEB(map[string][]byte{
		"elasticbeanstalk": []byte(`{"EnvironmentResources":{"Instances":[{"Id":"i-aaa"},{"Id":"i-bbb"}]}}`),
	})
	got, err := s.Instances(Target{Region: "ap-northeast-2"}, "api-beta-kr-j21")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "i-aaa" || got[0].Num != 1 ||
		got[1].Num != 2 || got[1].Label != "api-beta-kr-j21#2" {
		t.Errorf("instances 파싱 틀림: %+v", got)
	}
}
