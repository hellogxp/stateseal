package identity

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func Digest(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

func JSONDigest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return Digest(b), nil
}

func ID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func GitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a Git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

func Git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// CheckpointIdentity verifies that Git can resolve explicit author and
// committer identities without falling back to host-derived values. StateSeal
// performs this check before starting an Agent because every admitted
// candidate must be materialized as an attributable checkpoint commit.
func CheckpointIdentity(root string) error {
	for _, variable := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
		if _, err := Git(root, "-c", "user.useConfigOnly=true", "var", variable); err != nil {
			return fmt.Errorf("checkpoint commit requires a configured Git identity; set it for this repository with `git config --local user.name \"Your Name\"` and `git config --local user.email \"you@example.com\"`: %w", err)
		}
	}
	return nil
}

func EnsureLocalExclude(root, pattern string) error {
	out, err := Git(root, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n# StateSeal local state\n%s\n", pattern)
	return err
}

func Tree(root string, include []string) (string, error) {
	out, err := Git(root, "ls-files", "-co", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(out), "\x00")
	files := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || !included(p, include) {
			continue
		}
		files = append(files, p)
	}
	sort.Strings(files)
	h := sha256.New()
	w := bufio.NewWriter(h)
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(w, "%s\x00%o\x00", filepath.ToSlash(rel), info.Mode())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			io.WriteString(w, target)
		} else {
			f, err := os.Open(path)
			if err != nil {
				return "", err
			}
			if _, err = io.Copy(w, f); err != nil {
				f.Close()
				return "", err
			}
			f.Close()
		}
		w.WriteByte(0)
	}
	w.Flush()
	return hex.EncodeToString(h.Sum(nil)), nil
}

func included(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if pattern == "**" {
			return true
		}
		ok, _ := doublestar.Match(pattern, filepath.ToSlash(path))
		if ok {
			return true
		}
	}
	return false
}

func MatchesAny(path string, patterns []string) bool {
	for _, p := range patterns {
		ok, _ := doublestar.Match(p, filepath.ToSlash(path))
		if ok {
			return true
		}
	}
	return false
}

func UnsafeSymlinks(root string) ([]string, error) {
	out, err := Git(root, "ls-files", "-co", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	indexSymlinks, err := gitIndexSymlinks(root)
	if err != nil {
		return nil, err
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var unsafe []string
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		isFilesystemSymlink := info.Mode()&os.ModeSymlink != 0
		if !isFilesystemSymlink && !indexSymlinks[rel] {
			continue
		}
		var target string
		if isFilesystemSymlink {
			target, err = os.Readlink(path)
		} else {
			// Git materializes index mode 120000 as a regular file when
			// core.symlinks=false. Its contents are still the link target and
			// must be checked so a repository escape cannot become active on
			// another checkout or host.
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, readErr
			}
			target = string(data)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		target, err = filepath.Abs(filepath.Clean(target))
		if err != nil {
			return nil, err
		}
		relToRoot, err := filepath.Rel(cleanRoot, target)
		if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
			unsafe = append(unsafe, filepath.ToSlash(rel))
		}
	}
	return unsafe, nil
}

func gitIndexSymlinks(root string) (map[string]bool, error) {
	out, err := Git(root, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	links := make(map[string]bool)
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		metadata, path, ok := strings.Cut(entry, "\t")
		if !ok {
			return nil, fmt.Errorf("malformed git index entry")
		}
		mode, _, ok := strings.Cut(metadata, " ")
		if !ok {
			return nil, fmt.Errorf("malformed git index metadata for %q", path)
		}
		if mode == "120000" {
			links[path] = true
		}
	}
	return links, nil
}
