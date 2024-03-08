package coverage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/go-git/go-git/v5"
	"github.com/iszk1215/mora/mora/base"
	"github.com/iszk1215/mora/mora/profile"
)

func listRepositories(baseURL string, token string) ([]base.Repository, error) {
	log.Print("listRepositories")

	url := fmt.Sprintf("%s%s", baseURL, "/api/repos")
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var repos []base.Repository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}

	return repos, nil
}

func findRepositoryByURL(baseURL, repoURL string) (*base.Repository, error) {
	token := os.Getenv("MORA_API_KEY")

	repos, err := listRepositories(baseURL, token)
	if err != nil {
		return nil, err
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
		return nil, err
	}
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
		return nil, err
	}

	err = replaceFileName(profiles, root)
	if err != nil {
		return nil, err
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
		return err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/repos/%d/coverages", serverURL, repo.Id)
	r, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	r.Header.Set("Authorization", "Bearer "+os.Getenv("MORA_API_KEY"))

	client := http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return err
	}

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		log.Print(msg)
		return errors.New("returned status is not StatusCreated")
	}

	return nil
}

func isDirty(repo *git.Repository) (bool, error) {
	w, err := repo.Worktree()
	if err != nil {
		return false, err
	}

	status, err := w.Status()
	if err != nil {
		return false, err
	}

	for _, s := range status {
		if s.Worktree == 'M' {
			return true, nil
		}
	}
	return false, nil
}

func checkRequest(req *CoverageUploadRequest, repo *git.Repository) (bool, error) {
	isDirty, err := isDirty(repo)
	if err != nil {
		return false, err
	}

	return !isDirty, nil
}

func makeRequest(repo *git.Repository, url, entryName string, files ...string) (*CoverageUploadRequest, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	root := os.DirFS(wt.Filesystem.Root())

	entries := []*CoverageEntryUploadRequest{}
	for _, file := range files {
		e, err := parseFile(file, entryName, root)
		if err != nil {
			return nil, err
		}

		entries = append(entries, e)
	}

	if url == "" {
		remote, err := repo.Remote("origin")
		if err != nil {
			return nil, err
		}
		log.Print(remote.Config().URLs)
		url = remote.Config().URLs[0]
		url = strings.TrimSuffix(url, ".git")
	}

	req := &CoverageUploadRequest{
		RepoURL:   url,
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

	fmt.Printf("%-20s%s\n", "Repository", req.RepoURL)
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
		return false, err
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
		// log.Fatal().Err(err).Msg("failed to make a request")
		return err
	}

	flag, err := checkRequest(req, repo)
	if err != nil {
		return err
	}

	if !force && !flag {
		fmt.Println("working tree is dirty")
		return err
	}

	printRequest(req)

	if !yes {
		ok, err := ask()
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Canceled")
			return nil
		}
	}

	if !dryRun {
		if server == "" {
			fmt.Println("use -server=<server url>")
			os.Exit(1)
		}

		err = upload(server, repoURL, req)
		if err != nil {
			return err
		}

		fmt.Println("Uploaded")
	}

	return nil
}
