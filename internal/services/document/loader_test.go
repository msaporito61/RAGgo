package document

import (
	"strings"
	"testing"
)

func TestLoadText_TXT(t *testing.T) {
	text, err := LoadText("test.txt", []byte("hello world\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !strings.Contains(text, "hello world") {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestLoadText_MD(t *testing.T) {
	text, err := LoadText("readme.md", []byte("# Title\n\nParagraph."))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !strings.Contains(text, "Title") {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestLoadText_UnsupportedFormat(t *testing.T) {
	_, err := LoadText("file.xyz", []byte("data"))
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}
