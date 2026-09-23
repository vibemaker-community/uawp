package adapter

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/vibemaker-community/uawp/internal/plan"
)

type BlockSpec struct {
	ArtifactID, Target string
	Consumers          []string
	Body               string
}
type BlockMeta struct{ OutsideSHA256 string }

func blockBytes(spec BlockSpec, nl string) []byte {
	consumers := append([]string(nil), spec.Consumers...)
	sort.Strings(consumers)
	return []byte(fmt.Sprintf("<!-- UAWP:BEGIN artifact=%s schema=1 target=%s consumers=%s -->%s%s%s<!-- UAWP:END artifact=%s -->", spec.ArtifactID, spec.Target, strings.Join(consumers, ","), nl, spec.Body, nl, spec.ArtifactID))
}

func UpsertManagedBlock(content []byte, spec BlockSpec) ([]byte, BlockMeta, error) {
	nl := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		nl = "\r\n"
	}
	prefix := []byte("<!-- UAWP:BEGIN")
	count := bytes.Count(content, prefix)
	if count > 1 || bytes.Count(content, []byte("<!-- UAWP:END")) > 1 {
		return nil, BlockMeta{}, fmt.Errorf("duplicate UAWP markers")
	}
	want := blockBytes(spec, nl)
	if count == 1 {
		start := bytes.Index(content, prefix)
		endPrefix := []byte("<!-- UAWP:END artifact=" + spec.ArtifactID + " -->")
		end := bytes.Index(content[start:], endPrefix)
		if end < 0 {
			return nil, BlockMeta{}, fmt.Errorf("missing or mismatched UAWP end marker")
		}
		end = start + end + len(endPrefix)
		if bytes.Contains(content[start+len(prefix):end-len(endPrefix)], prefix) {
			return nil, BlockMeta{}, fmt.Errorf("nested UAWP marker")
		}
		if !bytes.Equal(content[start:end], want) {
			return nil, BlockMeta{}, fmt.Errorf("managed block drift")
		}
		outside := append(append([]byte(nil), content[:start]...), content[end:]...)
		return append([]byte(nil), content...), BlockMeta{OutsideSHA256: plan.HashBytes(stripBlockDelimiter(outside, start, nl))}, nil
	}
	if bytes.Contains(content, []byte("<!-- UAWP:END")) {
		return nil, BlockMeta{}, fmt.Errorf("orphan UAWP end marker")
	}
	out := append([]byte(nil), content...)
	if len(out) > 0 {
		out = append(out, []byte(nl)...)
	}
	out = append(out, want...)
	return out, BlockMeta{OutsideSHA256: plan.HashBytes(content)}, nil
}

func RemoveManagedBlock(content []byte, spec BlockSpec) ([]byte, error) {
	nl := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		nl = "\r\n"
	}
	want := blockBytes(spec, nl)
	start := bytes.Index(content, want)
	if start < 0 || bytes.Count(content, []byte("<!-- UAWP:BEGIN")) != 1 || bytes.Count(content, []byte("<!-- UAWP:END")) != 1 {
		return nil, fmt.Errorf("managed block missing or drifted")
	}
	end := start + len(want)
	out := append(append([]byte(nil), content[:start]...), content[end:]...)
	return stripBlockDelimiter(out, start, nl), nil
}

func stripBlockDelimiter(content []byte, start int, nl string) []byte {
	if start >= len(nl) && start <= len(content) {
		return append(append([]byte(nil), content[:start-len(nl)]...), content[start:]...)
	}
	return content
}
