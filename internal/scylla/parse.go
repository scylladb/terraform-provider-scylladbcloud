package scylla

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"net"
	"strconv"
	"strings"
)

//go:embed codes.txt
var codes []byte

var (
	codesDelim = " "
	codesFunc  = func(k, v string) (string, string, error) {
		if _, err := strconv.Atoi(k); err != nil {
			return "", "", fmt.Errorf("error parsing %q code: %w", k, err)
		}
		return k, v, nil
	}
)

//go:embed blocks.txt
var blocks []byte

var (
	blocksDelim = "\t"
	blocksFunc  = func(k, v string) (string, string, error) {
		p := strings.SplitN(v, blocksDelim, 2)
		if len(p) != 2 {
			return "", "", fmt.Errorf("unable to parse cidr line: %q", v)
		}

		if _, _, err := net.ParseCIDR(p[0]); err != nil {
			return "", "", fmt.Errorf("unable to parse %q cidr: %w", p[0], err)
		}

		return k, p[0], nil
	}
)

type mapFunc func(k, v string) (string, string, error)

func parse(p []byte, delim string, fn mapFunc) (map[string]string, error) {
	var (
		s = bufio.NewScanner(bytes.NewReader(p))
		m = make(map[string]string)
	)

	for s.Scan() {
		if strings.HasPrefix(s.Text(), "#") {
			continue
		}

		p := strings.SplitN(s.Text(), delim, 2)
		if len(p) != 2 {
			return nil, fmt.Errorf("unable to parse line: %q", s.Text())
		}

		var (
			key   = strings.TrimSpace(p[0])
			value = strings.TrimSpace(p[1])
		)

		if key == "" || value == "" {
			return nil, fmt.Errorf("unable to parse line: %q (key=%q, value=%q)", s.Text(), key, value)
		}

		if fn != nil {
			var err error
			if key, value, err = fn(key, value); err != nil {
				return nil, err
			}
		}

		m[key] = value
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return m, nil
}
