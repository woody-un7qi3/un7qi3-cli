package output

import (
	"os"
	"regexp"
	"strings"

	"golang.org/x/term"
)

// flagLineRe matches one cobra flag-usage line:
//
//	"      --config string   설정 파일 경로"
//	"  -h, --help            도움말 표시"
//
// Group 1: leading whitespace
// Group 2: flag spec (incl. optional short, long, and type indicator)
// Group 3: gap of >=2 spaces
// Group 4: description (may be empty)
var flagLineRe = regexp.MustCompile(`^(\s+)((?:-\w, )?--\S+(?: \S+)?)(\s{2,})(.*)$`)

// flagPartsRe splits a flag spec into its short and long parts.
//
//	"-h, --help"          → short="-h", long="--help"
//	"-v, --verbose"       → short="-v", long="--verbose"
//	"--config string"     → short="",   long="--config string"
//	"--json"              → short="",   long="--json"
var flagPartsRe = regexp.MustCompile(`^(?:(-\w), )?(--\S+(?: \S+)?)$`)

// cmdLineRe matches one cobra subcommand-listing line:
//
//	"  login       gh + aws + gcloud 로그인 (전체 또는 선택)"
//
// Group 1: leading whitespace (>=2)
// Group 2: command name (no spaces)
// Group 3: gap of >=2 spaces
// Group 4: short description
var cmdLineRe = regexp.MustCompile(`^(\s{2,})(\S+)(\s{2,})(.*)$`)

// Glyphs used in human-friendly TTY output.
const (
	GlyphOK       = "✓"
	GlyphFail     = "✗"
	GlyphOptional = "-"
)

// ANSI SGR codes.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiItalic = "\033[3m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiCyan   = "\033[36m"
)

// colorsEnabled returns true when ANSI escape codes are safe to emit.
// Respects NO_COLOR (https://no-color.org/) and TERM=dumb, and requires
// stdout to be a TTY.
func colorsEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func wrap(code, s string) string {
	if !colorsEnabled() {
		return s
	}
	return code + s + ansiReset
}

// Color helpers for human-friendly text rendering.
func Bold(s string) string   { return wrap(ansiBold, s) }
func Dim(s string) string    { return wrap(ansiDim, s) }
func Italic(s string) string { return wrap(ansiItalic, s) }
func Cyan(s string) string   { return wrap(ansiCyan, s) }
func Yellow(s string) string { return wrap(ansiYellow, s) }
func Green(s string) string  { return wrap(ansiGreen, s) }
func Red(s string) string    { return wrap(ansiRed, s) }
func Blue(s string) string   { return wrap(ansiBlue, s) }

// Desc renders a description / intro line — dimmed so it reads as
// secondary, informational text (not actionable like commands/flags).
func Desc(s string) string {
	return wrap(ansiDim, s)
}

// parenHintRe matches a parenthetical hint anywhere in a line, e.g.
// "(gh, aws, gcloud)" or "(인증 점검 + 워크스페이스 위치)".
var parenHintRe = regexp.MustCompile(`\([^)]*\)`)

// DimParenHints drops the parenthesis characters from a parenthetical hint and
// dims the inner text, so it reads as a quiet secondary detail after the main
// description in --help command listings. With color disabled the parentheses
// are still removed (the inner text is returned plain).
//
//	"인증 관리 (gh, aws, gcloud)" → "인증 관리 " + dim "gh, aws, gcloud"
func DimParenHints(s string) string {
	return parenHintRe.ReplaceAllStringFunc(s, func(m string) string {
		return Dim(m[1 : len(m)-1]) // inner text, sans the ASCII '(' ')' bytes
	})
}

// Heading renders a section heading (bold + a soft underline glyph).
func Heading(s string) string {
	return Bold(s)
}

// Section renders a consistent step/section header for multi-step interactive
// flows (e.g. `uq init`). Centralizing the format here means new steps extend
// uniformly without each call site re-inventing a divider.
//
//	▌ 1. 인증
func Section(s string) string {
	return Cyan("▌") + " " + Bold(s)
}

// HelpExample renders a single help-example line. cmd is colorized as a
// command, note is dimmed.
//
//	$ uq auth login --gh-only        gh만
func HelpExample(cmd, note string) string {
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(Dim("$"))
	b.WriteString(" ")
	b.WriteString(Cyan(cmd))
	if note != "" {
		// Pad command to a stable column for alignment.
		pad := 36 - visibleLen(cmd)
		if pad < 2 {
			pad = 2
		}
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(Dim(note))
	}
	return b.String()
}

// HelpFlag renders a flag-list line with the flag in yellow.
func HelpFlag(flag, note string) string {
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(Yellow(flag))
	pad := 44 - visibleLen(flag)
	if pad < 2 {
		pad = 2
	}
	b.WriteString(strings.Repeat(" ", pad))
	b.WriteString(note)
	return b.String()
}

// visibleLen returns the rune length of s (ANSI codes are stripped by
// callers; this is only used for pre-styled plain strings).
func visibleLen(s string) int {
	return len([]rune(s))
}

// ColorizeFlagUsages takes cobra's `FlagUsages()` output, reorders so that
// the long flag always starts at the same column ("--help, -h" instead of
// "-h, --help"), then colorizes flag specs in yellow. Descriptions are
// left untouched. When colors are disabled, only the reordering happens.
func ColorizeFlagUsages(s string) string {
	type row struct{ long, short, desc string }
	rows := make([]row, 0)
	rawLines := strings.Split(s, "\n")

	// Pass 1: parse. Lines we can't parse are kept verbatim with empty long.
	for _, line := range rawLines {
		m := flagLineRe.FindStringSubmatch(line)
		if m == nil {
			rows = append(rows, row{desc: line})
			continue
		}
		spec := m[2]
		desc := strings.TrimLeft(line[len(m[1])+len(m[2])+len(m[3]):], "")
		parts := flagPartsRe.FindStringSubmatch(spec)
		if parts == nil {
			rows = append(rows, row{long: spec, desc: desc})
			continue
		}
		rows = append(rows, row{short: parts[1], long: parts[2], desc: desc})
	}

	// Compute padding: max width of "long[, -X]" across all parseable rows.
	maxWidth := 0
	for _, r := range rows {
		if r.long == "" {
			continue
		}
		w := len(r.long)
		if r.short != "" {
			w += len(", ") + len(r.short)
		}
		if w > maxWidth {
			maxWidth = w
		}
	}

	// Pass 2: render.
	var b strings.Builder
	for i, r := range rows {
		if r.long == "" {
			// Unparseable / verbatim line.
			b.WriteString(r.desc)
		} else {
			spec := r.long
			if r.short != "" {
				spec += ", " + r.short
			}
			pad := maxWidth - len(spec) + 2
			if pad < 2 {
				pad = 2
			}
			b.WriteString("  ")
			b.WriteString(wrap(ansiYellow, spec))
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(r.desc)
		}
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ColorizeCommandList takes the rendered command list block (one command
// per line, "  name   short description") and colorizes the command name
// (cyan). Short descriptions left untouched.
func ColorizeCommandList(s string) string {
	if !colorsEnabled() {
		return s
	}
	re := cmdLineRe
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// m[1] leading spaces, m[2] name, m[3] separator, m[4] short
		lines[i] = m[1] + wrap(ansiCyan, m[2]) + m[3] + m[4]
	}
	return strings.Join(lines, "\n")
}
