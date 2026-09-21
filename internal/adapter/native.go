package adapter

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/uawp/uawp/internal/plan"
)

const maxNativeBytes = 1 << 20

func DiscoverSnapshot(root string, candidates []string, version string, options map[string]string) (Snapshot, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, err
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Snapshot{}, err
	}
	s := Snapshot{Root: canonical, Files: map[string]FileFact{}, ProviderVersion: version, Options: map[string]string{}}
	for k, v := range options {
		s.Options[k] = v
	}
	for _, name := range candidates {
		if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") || path.Clean(name) != name || strings.HasPrefix(name, "../") {
			return Snapshot{}, fmt.Errorf("invalid native candidate %q", name)
		}
		full := filepath.Join(canonical, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		fact := FileFact{Path: name}
		if os.IsNotExist(err) {
			fact.State = FileMissing
			s.Files[name] = fact
			continue
		}
		if err != nil {
			return Snapshot{}, err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			fact.State = FileUnsafe
			s.Files[name] = fact
			continue
		}
		file, err := os.Open(full)
		if err != nil {
			return Snapshot{}, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxNativeBytes+1))
		_ = file.Close()
		if readErr != nil {
			return Snapshot{}, readErr
		}
		switch {
		case len(data) > maxNativeBytes:
			fact.State = FileTooLarge
		case strings.IndexByte(string(data), 0) >= 0:
			fact.State = FileBinary
		case !utf8.Valid(data):
			fact.State = FileInvalidUTF8
		default:
			fact.State = FileRegular
			fact.Content = append([]byte(nil), data...)
			fact.SHA256 = plan.HashBytes(data)
		}
		s.Files[name] = fact
	}
	return s, nil
}
