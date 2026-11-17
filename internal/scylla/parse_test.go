package scylla

import (
	"testing"
)

func TestParseCodes(t *testing.T) {
	m, err := parse(codes, codesDelim, codesFunc)
	if err != nil {
		t.Fatalf("parse()=%+v", err)
	}

	if len(m) == 0 {
		t.Fatalf("want len(m) != %d, got %d", 0, len(m))
	}
}

// TestParseCodesPeerRegion checks the codes the API returns for a bad peer
// region. Without a message, the error only says to consult the error code.
func TestParseCodesPeerRegion(t *testing.T) {
	m, err := parse(codes, codesDelim, codesFunc)
	if err != nil {
		t.Fatalf("parse()=%+v", err)
	}

	for _, code := range []string{"041115", "041122", "041123", "041124", "041125"} {
		if m[code] == "" {
			t.Errorf("no message for code %s", code)
		}
	}
}

func TestParseCIDRBlocks(t *testing.T) {
	m, err := parse(blocks, blocksDelim, blocksFunc)
	if err != nil {
		t.Fatalf("parse()=%+v", err)
	}

	if len(m) == 0 {
		t.Fatalf("want len(m) != %d, got %d", 0, len(m))
	}
}
