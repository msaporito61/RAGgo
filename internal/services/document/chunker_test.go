package document

import (
	"strings"
	"testing"
)

func TestChunk_SmallText(t *testing.T) {
	chunks := Chunk("hello world", 1000, 200)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello world" {
		t.Errorf("unexpected chunk: %q", chunks[0])
	}
}

func TestChunk_Overlap(t *testing.T) {
	// 50-char string, size=30, overlap=10
	text := strings.Repeat("abcdefghij", 5) // 50 chars
	chunks := Chunk(text, 30, 10)
	if len(chunks) < 2 {
		t.Fatalf("want at least 2 chunks, got %d", len(chunks))
	}
	// second chunk should start at position 20 (30-10)
	if !strings.HasPrefix(chunks[1], text[20:30]) {
		t.Errorf("overlap not applied correctly; chunk[1] = %q", chunks[1])
	}
}

func TestChunk_EmptyText(t *testing.T) {
	chunks := Chunk("", 1000, 200)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}
