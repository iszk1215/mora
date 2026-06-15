package coverage

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/go-git/go-git/v5"
	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/coverage/profile"
)

type (
	coverageClient struct {
		client *core.APIClientImpl
	}
)

func (c *coverageClient) listRepositories() ([]core.Repository, error) {
	return c.client.ListRepositories()
}

func findRepositoryByURL(baseURL, repoURL string) (*core.Repository, error) {
	token := os.Getenv("MORA_API_KEY")

	client := coverageClient{
		client: &core.APIClientImpl{
			BaseURL: baseURL,
			Token:   token,
			Client:  &http.Client{},
		},
	}

	repos, err := client.listRepositories()
	if err != nil {
		return nil, fmt.Errorf("findRepositoryByURL listRepositories: %w", err)
	}

	for _, r := range repos {
		if r.Url == repoURL {
			return &r, nil
		}
	}

	return nil, errors.New("no repository found")
}

// ----------------------------------------------------------------------

func parseCoverageFromFile(filename string) ([]*profile.Profile, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("parseCoverageFromFile open %s: %w", filename, err)
	}
	defer reader.Close()
	return profile.ParseCoverage(reader)
}

func relativePathFromRoot(path string, root fs.FS) string {
	lst := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for i := range lst {
		relativePath := filepath.Join(lst[i:]...)
		_, err := fs.Stat(root, relativePath)
		if !os.IsNotExist(err) {
			return relativePath
		}
	}
	return ""
}

func replaceFileName(profiles []*profile.Profile, root fs.FS) error {
	for _, p := range profiles {
		file := relativePathFromRoot(p.FileName, root)
		if file == "" {
			return fmt.Errorf("file not found: %s", p.FileName)
		}
		p.FileName = file
	}
	return nil
}

func parseFile(filename, entryName string, root fs.FS) (*CoverageEntryUploadRequest, error) {
	profiles, err := parseCoverageFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("parseFile(%s): %w", filename, err)
	}

	err = replaceFileName(profiles, root)
	if err != nil {
		return nil, fmt.Errorf("parseFile replaceFileName: %w", err)
	}

	hits := 0
	lines := 0
	for _, p := range profiles {
		hits += p.Hits
		lines += p.Lines
	}

	e := &CoverageEntryUploadRequest{
		Name:     entryName,
		Profiles: profiles,
		Hits:     hits,
		Lines:    lines,
	}

	return e, nil
}

func upload(serverURL, repoURL string, req *CoverageUploadRequest) error {
	repo, err := findRepositoryByURL(serverURL, repoURL)
	if err != nil {
		return fmt.Errorf("upload findRepositoryByURL: %w", err)
	}

	client := coverageClient{
		client: &core.APIClientImpl{
			BaseURL: serverURL,
			Token:   os.Getenv("MORA_API_KEY"),
			Client:  &http.Client{},
		},
	}

	url := fmt.Sprintf("/api/repos/%d/coverages", repo.Id)
	err = client.client.Do(http.MethodPost, url, &req, nil)
	if err != nil {
		return fmt.Errorf("upload client.Do: %w", err)
	}

	return nil
}

func isDirty(repo *git.Repository) (bool, error) {
	w, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("isDirty Worktree: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return false, fmt.Errorf("isDirty Status: %w", err)
	}

	for _, s := range status {
		if s.Worktree == 'M' {
			return true, nil
		}
	}
	return false, nil
}

func checkRequest(req *CoverageUploadRequest, repo *git.Repository) (bool, error) {
	dirty, err := isDirty(repo)
	if err != nil {
		return false, err
	}

	return !dirty, nil
}

func makeRequest(repo *git.Repository, url, entryName string, files ...string) (*CoverageUploadRequest, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("makeRequest repo.Head: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("makeRequest CommitObject: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("makeRequest Worktree: %w", err)
	}
	root := os.DirFS(wt.Filesystem.Root())

	entries := []*CoverageEntryUploadRequest{}
	for _, file := range files {
		e, err := parseFile(file, entryName, root)
		if err != nil {
			return nil, fmt.Errorf("makeRequest parseFile %s: %w", file, err)
		}

		entries = append(entries, e)
	}

	req := &CoverageUploadRequest{
		Revision:  commit.Hash.String(),
		Timestamp: commit.Committer.When,
		Entries:   entries,
	}

	return req, nil
}

type stats struct {
	Hits  int
	Lines int
}

func NewStats() *stats {
	return &stats{0, 0}
}

func (s *stats) Add(hits, lines int) {
	s.Hits += hits
	s.Lines += lines
}

func printRequest(req *CoverageUploadRequest) {
	nfiles := 0
	s := NewStats()
	for _, e := range req.Entries {
		s.Add(e.Hits, e.Lines)
		nfiles += len(e.Profiles)
	}

	fmt.Printf("%-20s%s\n", "Revision", req.Revision)
	fmt.Printf("%-20s%s\n", "Time:", req.Timestamp)
	fmt.Printf("%-20s%.1f%% (%d Hit / %d Lines, %d Files)\n", "Coverage",
		float64(s.Hits)*100.0/float64(s.Lines), s.Hits, s.Lines, nfiles)

}

func ask() (bool, error) {
	fmt.Print("OK? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	ru, _, err := reader.ReadRune()
	if err != nil {
		return false, fmt.Errorf("ask reader.ReadRune: %w", err)
	}
	lru := unicode.ToLower(ru)
	if lru != rune('y') && lru != rune('\n') {
		return false, nil
	}
	return true, nil
}

func Upload(server, repoURL, repoPath, entryName string, dryRun, force bool, yes bool, args []string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return errors.New("can not open repository. Use -repo-path=<repository>")
	}

	req, err := makeRequest(repo, repoURL, entryName, args...)
	if err != nil {
		return fmt.Errorf("Upload makeRequest: %w", err)
	}

	flag, err := checkRequest(req, repo)
	if err != nil {
		return fmt.Errorf("Upload checkRequest: %w", err)
	}

	if !force && !flag {
		return fmt.Errorf("working tree is dirty")
	}

	printRequest(req)

	if !yes {
		ok, err := ask()
		if err != nil {
			return fmt.Errorf("Upload ask: %w", err)
		}
		if !ok {
			fmt.Println("Canceled")
			return nil
		}
	}

	if !dryRun {
		if server == "" {
			return fmt.Errorf("use -server=<server url>")
		}

		err = upload(server, repoURL, req)
		if err != nil {
			return fmt.Errorf("Upload upload: %w", err)
		}

		fmt.Println("Uploaded")
	}

	return nil
}
