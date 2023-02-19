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

	t.Log(m)
}

func TestParseCIDRBlocks(t *testing.T) {
	m, err := parse(blocks, blocksDelim, blocksFunc)
	if err != nil {
		t.Fatalf("parse()=%+v", err)
	}

	if len(m) == 0 {
		t.Fatalf("want len(m) != %d, got %d", 0, len(m))
	}

	t.Log(m)
}
