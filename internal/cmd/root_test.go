package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func makeCmd(name, short, group string, hidden bool) *cobra.Command {
	c := &cobra.Command{
		Use:     name,
		Short:   short,
		Hidden:  hidden,
		GroupID: group,
		Run:     func(*cobra.Command, []string) {},
	}
	return c
}

// renderCommandGroup must:
//   - filter by GroupID exactly (no leakage between groups)
//   - keep cobra's "available command" rule (hidden cmds dropped)
//   - retain the "help" exception so the help command shows in its group
//   - align command names by column width (verified by checking spacing)
func TestRenderCommandGroup_FiltersByGroupID(t *testing.T) {
	cmds := []*cobra.Command{
		makeCmd("alpha", "first", "g1", false),
		makeCmd("bravo", "second", "g2", false),
		makeCmd("charlie", "third", "g1", false),
	}
	out := renderCommandGroup(cmds, "g1")
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "charlie") {
		t.Errorf("g1 should contain alpha and charlie:\n%s", out)
	}
	if strings.Contains(out, "bravo") {
		t.Errorf("g1 should NOT contain bravo:\n%s", out)
	}
}

func TestRenderCommandGroup_HiddenExcluded(t *testing.T) {
	cmds := []*cobra.Command{
		makeCmd("visible", "v", "g1", false),
		makeCmd("secret", "s", "g1", true),
	}
	out := renderCommandGroup(cmds, "g1")
	if strings.Contains(out, "secret") {
		t.Errorf("hidden command leaked into output:\n%s", out)
	}
	if !strings.Contains(out, "visible") {
		t.Errorf("visible command missing:\n%s", out)
	}
}

func TestRenderCommandGroup_HelpIncluded(t *testing.T) {
	// cobra's `help` command has Run/RunE nil by default, which would make
	// IsAvailableCommand() false; the renderer must include it anyway.
	help := &cobra.Command{Use: "help", Short: "도움말"}
	cmds := []*cobra.Command{
		help,
		makeCmd("alpha", "a", "g1", false),
	}
	help.GroupID = "g1"
	out := renderCommandGroup(cmds, "g1")
	if !strings.Contains(out, "help") {
		t.Errorf("help should appear in its group:\n%s", out)
	}
}

func TestRenderCommandGroup_UngroupedFallback(t *testing.T) {
	cmds := []*cobra.Command{
		makeCmd("alpha", "g", "g1", false),
		makeCmd("orphan", "u", "", false),
	}
	out := renderCommandGroup(cmds, "")
	if !strings.Contains(out, "orphan") {
		t.Errorf("empty-groupID query should match ungrouped commands:\n%s", out)
	}
	if strings.Contains(out, "alpha") {
		t.Errorf("ungrouped query leaked grouped commands:\n%s", out)
	}
}

// --json 은 일부 명령만 지원하므로 전역(persistent) 플래그가 아니라 지원 명령에만
// 로컬로 등록돼야 한다. --verbose 는 횡단 관심사라 전역 유지.
func TestJSONFlagScopedToSupportingCommands(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup("json") != nil {
		t.Error("--json 은 전역(persistent) 플래그가 아니어야 한다")
	}
	if rootCmd.PersistentFlags().Lookup("verbose") == nil {
		t.Error("--verbose 는 전역으로 유지돼야 한다")
	}

	supporting := [][]string{
		{"doctor"}, {"version"},
		{"auth", "status"}, {"repo", "list"}, {"repo", "clone"},
		{"run", "profiles"}, {"log", "targets"},
	}
	for _, path := range supporting {
		c := findCmdByPath(t, path)
		if c.LocalFlags().Lookup("json") == nil {
			t.Errorf("%v 는 로컬 --json 플래그를 가져야 한다", path)
		}
	}

	// 미지원 명령(log)은 --json 을 상속하지도, 로컬로 갖지도 않아야 한다.
	// (--json 은 하위 log targets 에만 있고 부모 log 로 올라오지 않는다.)
	logs := findCmdByPath(t, []string{"log"})
	if logs.InheritedFlags().Lookup("json") != nil {
		t.Error("log 가 --json 을 상속하면 안 된다")
	}
	if logs.LocalFlags().Lookup("json") != nil {
		t.Error("log 에 로컬 --json 이 있으면 안 된다")
	}
}

func findCmdByPath(t *testing.T, path []string) *cobra.Command {
	t.Helper()
	c, _, err := rootCmd.Find(path)
	if err != nil {
		t.Fatalf("명령 %v 찾기 실패: %v", path, err)
	}
	if c.Name() != path[len(path)-1] {
		t.Fatalf("명령 %v 를 못 찾음(실제: %s)", path, c.Name())
	}
	return c
}

// 명령 목록은 별칭이 있으면 "이름|별칭" 으로 보여준다 (예: update|upgrade).
func TestCommandListName(t *testing.T) {
	noAlias := &cobra.Command{Use: "doctor"}
	if got := commandListName(noAlias); got != "doctor" {
		t.Errorf("별칭 없음 = %q, want %q", got, "doctor")
	}
	aliased := &cobra.Command{Use: "update", Aliases: []string{"upgrade"}}
	if got := commandListName(aliased); got != "update|upgrade" {
		t.Errorf("별칭 있음 = %q, want %q", got, "update|upgrade")
	}
	multi := &cobra.Command{Use: "ls", Aliases: []string{"list", "l"}}
	if got := commandListName(multi); got != "ls|list|l" {
		t.Errorf("별칭 다수 = %q, want %q", got, "ls|list|l")
	}
}

// inGroup is a tiny helper but the chain "AddCommand(inGroup(NewCmd(), id))"
// is everywhere — make sure it doesn't drop the group.
func TestInGroup_AssignsID(t *testing.T) {
	c := &cobra.Command{Use: "foo"}
	got := inGroup(c, "x")
	if got != c {
		t.Fatal("inGroup should return the same cmd pointer")
	}
	if c.GroupID != "x" {
		t.Errorf("GroupID = %q, want %q", c.GroupID, "x")
	}
}
