package scylla

import "testing"

func TestParseCodes(t *testing.T) {
	m, err := parseCodes(codes)
	if err != nil {
		t.Fatalf("parseCodes()=%+v", err)
	}

	if len(m) == 0 {
		t.Fatalf("want len(m) != %d, got %d", 0, len(m))
	}

	t.Log(m)
}
