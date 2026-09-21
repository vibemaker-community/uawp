package adapter

import (
	"bytes"
	"fmt"
	"strings"
)

func UpsertImport(content []byte, target string) ([]byte, bool, error) {
	if target == "" || strings.ContainsAny(target, "\r\n") {
		return nil, false, fmt.Errorf("invalid import target")
	}
	count, first := importLines(content, target)
	if count > 1 {
		return nil, false, fmt.Errorf("duplicate import for %s", target)
	}
	if first >= 0 {
		return append([]byte(nil), content...), false, nil
	}
	nl := []byte("\n")
	if bytes.Contains(content, []byte("\r\n")) {
		nl = []byte("\r\n")
	}
	out := append([]byte(nil), content...)
	if len(out) > 0 {
		out = append(out, nl...)
	}
	out = append(out, []byte("@"+target)...)
	return out, true, nil
}

func RemoveImport(content []byte, target string) ([]byte, bool, error) {
	count, start := importLines(content, target)
	if count > 1 {
		return nil, false, fmt.Errorf("duplicate import for %s", target)
	}
	if start < 0 {
		return append([]byte(nil), content...), false, nil
	}
	end := start
	for end < len(content) && content[end] != '\n' {
		end++
	}
	if end < len(content) {
		end++
	}
	if start > 0 {
		if content[start-1] == '\n' {
			start--
			if start > 0 && content[start-1] == '\r' {
				start--
			}
		}
	}
	out := append(append([]byte(nil), content[:start]...), content[end:]...)
	return out, true, nil
}

func importLine(content []byte, target string) int {
	_, first := importLines(content, target)
	return first
}

func importLines(content []byte, target string) (int, int) {
	lines := bytes.SplitAfter(content, []byte("\n"))
	offset := 0
	fenced := false
	count, first := 0, -1
	for _, raw := range lines {
		line := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		} else if !fenced && line == "@"+target {
			count++
			if first < 0 {
				first = offset
			}
		}
		offset += len(raw)
	}
	return count, first
}
