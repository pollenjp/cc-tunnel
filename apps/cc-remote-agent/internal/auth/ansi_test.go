package auth

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "color code stripped",
			input: "\x1b[38;5;231mhello\x1b[0m",
			want:  "hello",
		},
		{
			name:  "cursor movement stripped",
			input: "\x1b[1Chello",
			want:  "hello",
		},
		{
			name:  "combined code stripped",
			input: "\x1b[1;2mhello",
			want:  "hello",
		},
		{
			name:  "complex sequence from PTY",
			input: "\x1b[1C\x1b[38;5;231m\x1b[2m 1\x1b[1C\x1b[22m \x1b[38;5;81mfunction\x1b[38;5;231m",
			want:  " 1 function",
		},
		{
			name:  "reset code stripped",
			input: "\x1b[0mtext\x1b[0m",
			want:  "text",
		},
		{
			name:  "multiple sequences",
			input: "\x1b[2m\x1b[38;5;148mgreet\x1b[38;5;231m()\x1b[1C{\x1b[39m",
			want:  "greet(){",
		},
		{
			name:  "newline preserved",
			input: "line1\nline2",
			want:  "line1\nline2",
		},
		{
			name:  "OSC sequence stripped",
			input: "\x1b]0;title\x07text",
			want:  "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
