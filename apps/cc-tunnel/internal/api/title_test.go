package api

import "testing"

func TestGenerateTitle_shortText(t *testing.T) {
	got := generateTitle("Hello World")
	want := "Hello World"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_emptyText(t *testing.T) {
	got := generateTitle("")
	want := "New Conversation"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_whitespaceOnly(t *testing.T) {
	got := generateTitle("   \n\t  ")
	want := "New Conversation"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_truncatesAt60Chars(t *testing.T) {
	// 61 chars input → truncated to 60 + "..."
	input := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxy"
	got := generateTitle(input)
	want := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwx..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_exactly60Chars_noEllipsis(t *testing.T) {
	// exactly 60 chars → no ellipsis
	input := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwx"
	got := generateTitle(input)
	want := input
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_newlinesReplacedWithSpaces(t *testing.T) {
	got := generateTitle("Hello\nWorld\nFoo")
	want := "Hello World Foo"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_markdownHeadingRemoved(t *testing.T) {
	got := generateTitle("# My Title")
	want := "My Title"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_markdownBoldRemoved(t *testing.T) {
	got := generateTitle("**bold text**")
	want := "bold text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_markdownItalicRemoved(t *testing.T) {
	got := generateTitle("*italic text*")
	want := "italic text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_markdownCodeRemoved(t *testing.T) {
	got := generateTitle("`code`")
	want := "code"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateTitle_combined(t *testing.T) {
	// Newlines + markdown + truncation
	got := generateTitle("## Summary\nThis is a long description that exceeds sixty characters in total length")
	want := "Summary This is a long description that exceeds sixty charac..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
