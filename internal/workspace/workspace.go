package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hellogxp/stateseal/internal/identity"
)

const (
	defaultMaxDepth        = 6
	defaultMaxRepositories = 128
)

var skippedDirectories = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true,
	".cache": true, ".venv": true, "venv": true,
}

// Repository is one independently versioned Git root inside a user workspace.
// RelativePath is stable within the inspected workspace and is "." when the
// workspace itself is a repository.
type Repository struct {
	Name         string `json:"name"`
	Root         string `json:"root"`
	RelativePath string `json:"relative_path"`
}

// Inspection separates the folder a user opened from the repositories that
// StateSeal can bind to commits, policies, checkpoints, and receipts.
type Inspection struct {
	Root         string       `json:"root"`
	Repositories []Repository `json:"repositories"`
}

// Inspect discovers Git repositories below path without crossing into an
// already discovered repository. This deliberately treats nested repositories
// and submodules as part of their outer repository unless the user selects
// their path explicitly.
func Inspect(path string) (Inspection, error) {
	root, err := canonicalDirectory(path)
	if err != nil {
		return Inspection{}, err
	}
	if repositoryRoot, err := identity.GitRoot(root); err == nil {
		repositoryRoot = filepath.Clean(repositoryRoot)
		return Inspection{Root: repositoryRoot, Repositories: []Repository{{
			Name: filepath.Base(repositoryRoot), Root: repositoryRoot, RelativePath: ".",
		}}}, nil
	}

	inspection := Inspection{Root: root}
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrPermission) {
				return fs.SkipDir
			}
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if current != root {
			relative, relativeErr := filepath.Rel(root, current)
			if relativeErr != nil {
				return relativeErr
			}
			if depth(relative) > defaultMaxDepth || skippedDirectories[entry.Name()] {
				return fs.SkipDir
			}
		}
		if !hasGitMarker(current) {
			return nil
		}
		repositoryRoot, rootErr := identity.GitRoot(current)
		if rootErr != nil || filepath.Clean(repositoryRoot) != filepath.Clean(current) {
			return nil
		}
		relative, relativeErr := filepath.Rel(root, repositoryRoot)
		if relativeErr != nil {
			return relativeErr
		}
		inspection.Repositories = append(inspection.Repositories, Repository{
			Name: filepath.Base(repositoryRoot), Root: filepath.Clean(repositoryRoot),
			RelativePath: filepath.ToSlash(relative),
		})
		if len(inspection.Repositories) >= defaultMaxRepositories {
			return fmt.Errorf("workspace contains at least %d Git repositories; use an explicit --repo path", defaultMaxRepositories)
		}
		return fs.SkipDir
	})
	if err != nil {
		return Inspection{}, err
	}
	sort.Slice(inspection.Repositories, func(i, j int) bool {
		return inspection.Repositories[i].RelativePath < inspection.Repositories[j].RelativePath
	})
	return inspection, nil
}

// Resolve selects one repository from a Git directory or a non-Git workspace.
// A selector may be an absolute path, a path relative to the workspace, or an
// unambiguous repository name.
func Resolve(path, selector string) (string, error) {
	inspection, err := Inspect(path)
	if err != nil {
		return "", err
	}
	if len(inspection.Repositories) == 0 {
		return "", fmt.Errorf("workspace %s contains no Git repositories", inspection.Root)
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		if len(inspection.Repositories) == 1 {
			return inspection.Repositories[0].Root, nil
		}
		return "", ambiguousRepositoryError(inspection)
	}

	var selectedPath string
	if filepath.IsAbs(selector) {
		selectedPath, err = canonicalDirectory(selector)
	} else {
		selectedPath, err = canonicalDirectory(filepath.Join(inspection.Root, selector))
	}
	if err == nil {
		for _, repository := range inspection.Repositories {
			if samePath(repository.Root, selectedPath) {
				return repository.Root, nil
			}
		}
	}

	var matches []Repository
	for _, repository := range inspection.Repositories {
		if repository.Name == selector || repository.RelativePath == filepath.ToSlash(selector) {
			matches = append(matches, repository)
		}
	}
	if len(matches) == 1 {
		return matches[0].Root, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("repository selector %q is ambiguous; use a relative path", selector)
	}
	return "", fmt.Errorf("repository %q is not part of workspace %s", selector, inspection.Root)
}

func ambiguousRepositoryError(inspection Inspection) error {
	paths := make([]string, 0, len(inspection.Repositories))
	for _, repository := range inspection.Repositories {
		paths = append(paths, repository.RelativePath)
	}
	return fmt.Errorf("workspace %s contains multiple Git repositories (%s); select one with --repo or repository", inspection.Root, strings.Join(paths, ", "))
}

func canonicalDirectory(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", resolved)
	}
	return filepath.Clean(resolved), nil
}

func hasGitMarker(path string) bool {
	info, err := os.Lstat(filepath.Join(path, ".git"))
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}

func depth(relative string) int {
	if relative == "." || relative == "" {
		return 0
	}
	return len(strings.Split(filepath.Clean(relative), string(filepath.Separator)))
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
