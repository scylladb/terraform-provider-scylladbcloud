package scylla

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
)

//go:embed codes.txt
var codes []byte

func parseCodes(p []byte) (map[string]string, error) {
	var (
		s = bufio.NewScanner(bytes.NewReader(p))
		m = make(map[string]string)
	)

	for s.Scan() {
		p := strings.SplitN(s.Text(), " ", 2)
		if len(p) != 2 {
			return nil, fmt.Errorf("unable to parse line: %q", s.Text())
		}

		var (
			code = strings.TrimSpace(p[0])
			text = strings.TrimSpace(p[1])
		)

		if _, err := strconv.Atoi(code); err != nil {
			return nil, fmt.Errorf("unable to parse code %q: %w", code, err)
		}

		m[code] = text
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return m, nil
}
