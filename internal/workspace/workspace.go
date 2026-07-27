package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/hellogxp/stateseal/internal/identity"
)

const (
	defaultMaxDepth        = 6
	defaultMaxRepositories = 128
	maxRoutingTerms        = 16
	maxRoutingFiles        = 100
	maxRoutingBytes        = 4 << 20
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

// RouteCandidate records explainable, read-only evidence that a development
// goal belongs to one repository. Score is only meaningful relative to the
// other candidates in the same Decision.
type RouteCandidate struct {
	Repository   Repository `json:"repository"`
	Score        int        `json:"score"`
	MatchedTerms []string   `json:"matched_terms,omitempty"`
	Evidence     []string   `json:"evidence,omitempty"`
}

// Decision separates repository discovery from repository routing. Selected is
// nil only when no repository exists or the available evidence is genuinely
// ambiguous. Ordinary users should not need to supply a Git path.
type Decision struct {
	Inspection Inspection       `json:"inspection"`
	Selected   *Repository      `json:"selected,omitempty"`
	Method     string           `json:"method,omitempty"`
	Confidence string           `json:"confidence,omitempty"`
	Evidence   []string         `json:"evidence,omitempty"`
	Candidates []RouteCandidate `json:"candidates,omitempty"`
}

// Inspect discovers Git repositories below path without crossing into an
// already discovered repository. This deliberately treats nested repositories
// and submodules as part of their outer repository. Opening a nested repository
// directly makes that repository the workspace target.
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
	decision, err := Route(path, selector, "")
	if err != nil {
		return "", err
	}
	if decision.Selected != nil {
		return decision.Selected.Root, nil
	}
	if len(decision.Inspection.Repositories) == 0 {
		return "", fmt.Errorf("workspace %s contains no Git repositories", decision.Inspection.Root)
	}
	return "", ambiguousRepositoryError(decision.Inspection)
}

// Route selects a repository from an opened folder using, in order: an
// explicit automation override, a sole discovered repository, an exact
// repository mention in the goal, and unique read-only path/content evidence.
// It never silently picks the first repository.
func Route(path, selector, goal string) (Decision, error) {
	inspection, err := Inspect(path)
	if err != nil {
		return Decision{}, err
	}
	decision := Decision{Inspection: inspection}
	if len(inspection.Repositories) == 0 {
		return decision, nil
	}

	selector = strings.TrimSpace(selector)
	if selector != "" {
		selected, selectErr := selectRepository(inspection, selector)
		if selectErr != nil {
			return Decision{}, selectErr
		}
		decision.Selected = &selected
		decision.Method = "explicit_override"
		decision.Confidence = "certain"
		decision.Evidence = []string{fmt.Sprintf("automation override selected %s", selected.RelativePath)}
		return decision, nil
	}
	if len(inspection.Repositories) == 1 {
		selected := inspection.Repositories[0]
		decision.Selected = &selected
		decision.Method = "sole_repository"
		decision.Confidence = "certain"
		decision.Evidence = []string{"the opened workspace contains one repository"}
		return decision, nil
	}

	decision.Candidates = scoreRepositories(inspection.Repositories, goal)
	if len(decision.Candidates) == 0 || decision.Candidates[0].Score == 0 {
		return decision, nil
	}
	top := decision.Candidates[0]
	secondScore := 0
	if len(decision.Candidates) > 1 {
		secondScore = decision.Candidates[1].Score
	}
	directMention := top.Score >= 200
	uniqueEvidence := secondScore == 0 && top.Score >= 12
	dominantEvidence := top.Score >= 24 && top.Score >= secondScore*2 && top.Score-secondScore >= 18
	if !directMention && !uniqueEvidence && !dominantEvidence {
		return decision, nil
	}
	selected := top.Repository
	decision.Selected = &selected
	decision.Method = "goal_and_workspace_evidence"
	decision.Confidence = "medium"
	if directMention || top.Score >= 60 {
		decision.Confidence = "high"
	}
	decision.Evidence = append([]string(nil), top.Evidence...)
	return decision, nil
}

func selectRepository(inspection Inspection, selector string) (Repository, error) {
	var selectedPath string
	var err error
	if filepath.IsAbs(selector) {
		selectedPath, err = canonicalDirectory(selector)
	} else {
		selectedPath, err = canonicalDirectory(filepath.Join(inspection.Root, selector))
	}
	if err == nil {
		for _, repository := range inspection.Repositories {
			if samePath(repository.Root, selectedPath) {
				return repository, nil
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
		return matches[0], nil
	}
	if len(matches) > 1 {
		return Repository{}, fmt.Errorf("repository selector %q is ambiguous; use a relative path", selector)
	}
	return Repository{}, fmt.Errorf("repository %q is not part of workspace %s", selector, inspection.Root)
}

func scoreRepositories(repositories []Repository, goal string) []RouteCandidate {
	terms := routingTerms(goal)
	candidates := make([]RouteCandidate, 0, len(repositories))
	for _, repository := range repositories {
		candidate := RouteCandidate{Repository: repository}
		if repositoryMentioned(goal, repository) {
			candidate.Score += 200
			candidate.Evidence = append(candidate.Evidence,
				fmt.Sprintf("the goal names project %q", repository.RelativePath))
		}
		pathTerms, contentTerms, evidence := repositoryTermEvidence(repository, terms)
		candidate.Score += len(pathTerms)*30 + len(contentTerms)*12
		candidate.MatchedTerms = append(candidate.MatchedTerms, pathTerms...)
		for _, term := range contentTerms {
			if !contains(candidate.MatchedTerms, term) {
				candidate.MatchedTerms = append(candidate.MatchedTerms, term)
			}
		}
		candidate.Evidence = append(candidate.Evidence, evidence...)
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Repository.RelativePath < candidates[j].Repository.RelativePath
	})
	return candidates
}

func repositoryMentioned(goal string, repository Repository) bool {
	for _, value := range []string{repository.RelativePath, repository.Name} {
		value = strings.TrimSpace(value)
		if value == "" || value == "." || len([]rune(value)) < 3 {
			continue
		}
		if containsDelimitedFold(goal, value) {
			return true
		}
	}
	return false
}

func containsDelimitedFold(text, phrase string) bool {
	textRunes := []rune(strings.ToLower(text))
	phraseRunes := []rune(strings.ToLower(phrase))
	if len(phraseRunes) == 0 || len(phraseRunes) > len(textRunes) {
		return false
	}
	for index := 0; index+len(phraseRunes) <= len(textRunes); index++ {
		if string(textRunes[index:index+len(phraseRunes)]) != string(phraseRunes) {
			continue
		}
		beforeOK := index == 0 || !isRoutingWordRune(textRunes[index-1])
		after := index + len(phraseRunes)
		afterOK := after == len(textRunes) || !isRoutingWordRune(textRunes[after])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func repositoryTermEvidence(repository Repository, terms []string) ([]string, []string, []string) {
	if len(terms) == 0 {
		return nil, nil, nil
	}
	raw, err := identity.Git(repository.Root, "ls-files", "-co", "--exclude-standard", "-z")
	if err != nil {
		return nil, nil, nil
	}
	files := strings.Split(string(raw), "\x00")
	pathMatches := make(map[string]string)
	for _, file := range files {
		lowerPath := strings.ToLower(filepath.ToSlash(file))
		for _, term := range terms {
			if _, exists := pathMatches[term]; !exists && strings.Contains(lowerPath, term) {
				pathMatches[term] = filepath.ToSlash(file)
			}
		}
	}

	contentMatches := make(map[string]string)
	args := []string{"grep", "-I", "-l", "-i", "-z"}
	for _, term := range terms {
		args = append(args, "-e", term)
	}
	args = append(args, "--")
	command := exec.Command("git", args...)
	command.Dir = repository.Root
	matched, grepErr := command.Output()
	if grepErr == nil {
		var readBytes int64
		for index, file := range strings.Split(string(matched), "\x00") {
			if index >= maxRoutingFiles || readBytes >= maxRoutingBytes {
				break
			}
			if file == "" {
				continue
			}
			path := filepath.Join(repository.Root, filepath.FromSlash(file))
			info, statErr := os.Stat(path)
			if statErr != nil || !info.Mode().IsRegular() || info.Size() > 512<<10 {
				continue
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil || strings.IndexByte(string(content), 0) >= 0 {
				continue
			}
			readBytes += int64(len(content))
			lowerContent := strings.ToLower(string(content))
			for _, term := range terms {
				if _, exists := contentMatches[term]; !exists && strings.Contains(lowerContent, term) {
					contentMatches[term] = filepath.ToSlash(file)
				}
			}
		}
	}

	pathTerms := sortedKeys(pathMatches)
	contentTerms := sortedKeys(contentMatches)
	evidence := make([]string, 0, 4)
	for _, term := range pathTerms {
		if len(evidence) >= 4 {
			break
		}
		evidence = append(evidence, fmt.Sprintf("term %q matches path %s", term, pathMatches[term]))
	}
	for _, term := range contentTerms {
		if len(evidence) >= 4 || contains(pathTerms, term) {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("term %q matches content in %s", term, contentMatches[term]))
	}
	return pathTerms, contentTerms, evidence
}

func routingTerms(goal string) []string {
	stop := map[string]bool{
		"add": true, "and": true, "change": true, "code": true, "existing": true,
		"fix": true, "for": true, "implement": true, "method": true, "preserve": true,
		"test": true, "tests": true, "the": true, "update": true, "with": true,
		"一个": true, "代码": true, "保持": true, "修复": true, "兼容": true,
		"增加": true, "实现": true, "方法": true, "测试": true, "添加": true,
		"补充": true, "这个": true,
	}
	seen := make(map[string]bool)
	var terms []string
	var token []rune
	flush := func() {
		value := strings.ToLower(string(token))
		token = token[:0]
		if value == "" || stop[value] || seen[value] || len([]rune(value)) < 2 {
			return
		}
		if len([]rune(value)) == 2 && isASCII(value) {
			return
		}
		seen[value] = true
		terms = append(terms, value)
	}
	for _, char := range goal {
		if isRoutingWordRune(char) {
			token = append(token, char)
		} else {
			flush()
		}
	}
	flush()
	sort.SliceStable(terms, func(i, j int) bool {
		left, right := len([]rune(terms[i])), len([]rune(terms[j]))
		if left != right {
			return left > right
		}
		return terms[i] < terms[j]
	})
	if len(terms) > maxRoutingTerms {
		terms = terms[:maxRoutingTerms]
	}
	return terms
}

func isRoutingWordRune(char rune) bool {
	return unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_'
}

func isASCII(value string) bool {
	for _, char := range value {
		if char > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
