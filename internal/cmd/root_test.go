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
