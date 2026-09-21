package workspace

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

type Root struct {
	path string
}

func OpenRoot(value string) (Root, error) {
	abs, err := filepath.Abs(value)
	if err != nil {
		return Root{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Root{}, fmt.Errorf("resolve workspace root symlinks: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return Root{}, fmt.Errorf("inspect workspace root: %w", err)
	}
	if !info.IsDir() {
		return Root{}, fmt.Errorf("workspace root is not a directory: %s", canonical)
	}
	return Root{path: filepath.Clean(canonical)}, nil
}

func (r Root) Path() string {
	return r.path
}

func (r Root) ResolveUAWP(relative string) (string, error) {
	if r.path == "" {
		return "", fmt.Errorf("workspace root is empty")
	}
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, `\`) || strings.ContainsRune(relative, 0) {
		return "", fmt.Errorf("invalid UAWP relative path %q", relative)
	}
	clean := path.Clean(relative)
	if clean != relative || clean == "." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("UAWP path escapes namespace: %q", relative)
	}
	segments := strings.Split(clean, "/")
	for _, segment := range segments {
		if !validPortableSegment(segment) {
			return "", fmt.Errorf("non-portable UAWP path segment %q", segment)
		}
	}
	if !allowedUAWPPath(segments) {
		return "", fmt.Errorf("path is not UAWP-managed: %q", relative)
	}

	namespace := filepath.Join(r.path, ".uawp")
	target := filepath.Join(append([]string{namespace}, segments...)...)
	rel, err := filepath.Rel(r.path, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("resolved path escapes workspace: %q", relative)
	}
	if err := rejectSymlinkAncestors(r.path, target); err != nil {
		return "", err
	}
	return target, nil
}

func allowedUAWPPath(segments []string) bool {
	if len(segments) == 1 {
		switch segments[0] {
		case "manifest.json", "CONTEXT.md", "ACTIVE_WORKER.md", "DECISIONS.md", "RECOVERY.json", "checkpoints":
			return true
		}
	}
	return len(segments) == 2 && segments[0] == "checkpoints"
}

func rejectSymlinkAncestors(root, target string) error {
	current := filepath.Join(root, ".uawp")
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("UAWP mutation path contains symlink: %s", current)
			}
			if current != target && !info.IsDir() {
				return fmt.Errorf("UAWP mutation ancestor is not a directory: %s", current)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect UAWP path %s: %w", current, err)
		}
		if current == target {
			break
		}
		next := filepath.Join(current, firstRelativeSegment(current, target))
		if next == current {
			break
		}
		current = next
	}
	return nil
}

func firstRelativeSegment(current, target string) string {
	relative, err := filepath.Rel(current, target)
	if err != nil {
		return ""
	}
	return strings.Split(relative, string(filepath.Separator))[0]
}

func validPortableSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
		return false
	}
	if strings.ContainsAny(segment, `<>:"/\|?*`) {
		return false
	}
	for _, char := range segment {
		if unicode.IsControl(char) {
			return false
		}
	}
	base := strings.ToUpper(strings.SplitN(segment, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return false
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return false
	}
	return true
}
