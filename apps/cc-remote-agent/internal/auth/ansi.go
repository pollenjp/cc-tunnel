package auth

import "regexp"

// ansiEscapeRe matches ANSI/VT100 escape sequences:
//   - CSI sequences: ESC [ ... <final byte>
//   - OSC sequences: ESC ] ... BEL
var ansiEscapeRe = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07]*\x07)`)

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}
