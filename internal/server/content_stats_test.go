package server

import "testing"

func TestBuildContentStats_EmptyContent(t *testing.T) {
	stats := buildContentStats("")
	if stats.WordCount != 0 {
		t.Fatalf("WordCount = %d, want 0", stats.WordCount)
	}
	if stats.LineCount != 0 {
		t.Fatalf("LineCount = %d, want 0", stats.LineCount)
	}
	if stats.TokenCount != 0 {
		t.Fatalf("TokenCount = %d, want 0", stats.TokenCount)
	}
}

func TestBuildContentStats_WordsAndLines(t *testing.T) {
	stats := buildContentStats("one  two\nthree\r\nfour")
	if stats.WordCount != 4 {
		t.Fatalf("WordCount = %d, want 4", stats.WordCount)
	}
	if stats.LineCount != 3 {
		t.Fatalf("LineCount = %d, want 3", stats.LineCount)
	}
}

func TestCountTokens_TiktokenCompatibility(t *testing.T) {
	if got := countTokens("tiktoken is great!"); got != 6 {
		t.Fatalf("countTokens() = %d, want 6", got)
	}
}
