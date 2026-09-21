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
	if importLine(content, target) >= 0 {
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
	start := importLine(content, target)
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
	lines := bytes.SplitAfter(content, []byte("\n"))
	offset := 0
	fenced := false
	for _, raw := range lines {
		line := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		} else if !fenced && line == "@"+target {
			return offset
		}
		offset += len(raw)
	}
	return -1
}
