package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

// resolveCountry picks the locale variant for a profile.
//
// Returns nil (no error) when the profile has no country axis — callers then
// run it unchanged. Otherwise:
//
//   - flag set:   must name a declared option, else error (exit 1). Its
//     `requires` are pre-flighted.
//   - TTY:        interactive selection among options whose `requires` are met.
//   - non-TTY:    countries.default, with its `requires` pre-flighted.
//
// label is "<repo>:<profile>", used verbatim in the pre-flight error.
func resolveCountry(p repocfg.Profile, repoDir, flag, label string, isTTY bool, w io.Writer) (*repocfg.Country, error) {
	if p.Countries == nil {
		return nil, nil
	}
	cs := p.Countries

	if flag != "" {
		c, ok := cs.Find(flag)
		if !ok {
			return nil, fmt.Errorf("알 수 없는 국가 '%s' — 사용 가능: %s", flag, cs.Codes())
		}
		if err := preflight(repoDir, label, c); err != nil {
			return nil, err
		}
		return &c, nil
	}

	if isTTY {
		return selectCountryTUI(cs, repoDir, w)
	}

	c, ok := cs.Find(cs.Default)
	if !ok {
		return nil, fmt.Errorf("countries.default '%s' 가 options 에 없습니다", cs.Default)
	}
	if err := preflight(repoDir, label, c); err != nil {
		return nil, err
	}
	return &c, nil
}

// missingRequires returns the country's required files that don't exist under
// repoDir, as repo-root-relative paths (the form declared in repos.yml).
func missingRequires(repoDir string, c repocfg.Country) []string {
	var missing []string
	for _, rel := range c.Requires {
		if _, err := os.Stat(filepath.Join(repoDir, rel)); err != nil {
			missing = append(missing, rel)
		}
	}
	return missing
}

// preflight errors (exit 1) if any of the country's required files are absent.
func preflight(repoDir, label string, c repocfg.Country) error {
	if missing := missingRequires(repoDir, c); len(missing) > 0 {
		return fmt.Errorf("%s (%s) 실행 불가 — 없는 파일: %s", label, c.Code, joinComma(missing))
	}
	return nil
}

// selectCountryTUI lists countries whose requires are satisfied as selectable
// options (default pre-selected) and prints the blocked ones as dim, non-
// selectable lines. Errors if nothing is selectable.
func selectCountryTUI(cs *repocfg.Countries, repoDir string, w io.Writer) (*repocfg.Country, error) {
	var eligible []repocfg.Country
	type blockedEntry struct{ code, files string }
	var blocked []blockedEntry
	for _, c := range cs.Options {
		if m := missingRequires(repoDir, c); len(m) > 0 {
			blocked = append(blocked, blockedEntry{c.Code, joinComma(m)})
		} else {
			eligible = append(eligible, c)
		}
	}
	// 선택 불가 국가는 한 줄로 짧게 — "코드 — 없는 파일" 형식.
	for _, b := range blocked {
		fmt.Fprintln(w, output.Dim(fmt.Sprintf("⊘ %s — %s 없음", b.code, b.files)))
	}
	if len(blocked) > 0 {
		fmt.Fprintln(w)
	}
	if len(eligible) == 0 {
		return nil, fmt.Errorf("선택 가능한 국가가 없습니다 — 필요한 .env 파일이 모두 없습니다")
	}

	// 각 선택지에 그 국가가 읽을 .env 파일을 → 로 같이 보여준다.
	opts := make([]huh.Option[string], 0, len(eligible))
	for _, c := range eligible {
		label := c.Code
		if len(c.Requires) > 0 {
			label = fmt.Sprintf("%s → %s", c.Code, joinComma(c.Requires))
		}
		opts = append(opts, huh.NewOption(label, c.Code))
	}
	choice := eligible[0].Code
	if _, ok := cs.Find(cs.Default); ok {
		for _, c := range eligible {
			if c.Code == cs.Default {
				choice = cs.Default
				break
			}
		}
	}
	if err := huh.NewSelect[string]().
		Title("국가 선택").
		Description("→ 는 그 국가가 쓰는 .env").
		Options(opts...).
		Value(&choice).
		Run(); err != nil {
		return nil, err
	}
	c, _ := cs.Find(choice)
	return &c, nil
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
