package install

import (
	"fmt"
	"io"
	"strings"
)

// PrintUnifiedDiff writes a minimal unified diff (a → b) to w. Doesn't
// implement true LCS — just per-line comparison good enough for showing
// JSON config edits where most lines are unchanged. ANSI colors when
// stdout is a TTY (handled by caller).
//
// Header lines mimic `diff -u` so output looks familiar.
func PrintUnifiedDiff(w io.Writer, oldName, newName string, oldStr, newStr string, color bool) {
	fmt.Fprintf(w, "--- %s\n", oldName)
	fmt.Fprintf(w, "+++ %s\n", newName)

	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	// Naive LCS via shared-prefix + shared-suffix collapse, then per-line
	// diff in the middle. Good enough for JSON configs where edits are
	// localized.
	prefix := commonPrefixLen(oldLines, newLines)
	suffix := commonSuffixLen(oldLines[prefix:], newLines[prefix:])

	// Print prefix context (last 3 lines).
	ctxStart := prefix - 3
	if ctxStart < 0 {
		ctxStart = 0
	}
	for i := ctxStart; i < prefix; i++ {
		fmt.Fprintf(w, " %s\n", oldLines[i])
	}

	// Print removed.
	for _, l := range oldLines[prefix : len(oldLines)-suffix] {
		fmt.Fprintln(w, colorize("- "+l, "31", color))
	}
	// Print added.
	for _, l := range newLines[prefix : len(newLines)-suffix] {
		fmt.Fprintln(w, colorize("+ "+l, "32", color))
	}

	// Print suffix context (first 3 lines).
	ctxEnd := suffix
	if ctxEnd > 3 {
		ctxEnd = 3
	}
	for i := 0; i < ctxEnd; i++ {
		fmt.Fprintf(w, " %s\n", oldLines[len(oldLines)-suffix+i])
	}
}

func commonPrefixLen(a, b []string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func commonSuffixLen(a, b []string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[len(a)-1-i] != b[len(b)-1-i] {
			return i
		}
	}
	return n
}

func colorize(s, code string, on bool) string {
	if !on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
