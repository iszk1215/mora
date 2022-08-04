package upload

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/iszk1215/mora/mora/profile"
	"github.com/iszk1215/mora/mora/server"
)

func parseCoverageFromFile(filename string) ([]*profile.Profile, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return profile.ParseCoverage(reader)
}

func relativePathFromRoot(path string, root string) string {
	relativePath := ""
	for path != "." && path != "/" {
		relativePath = filepath.Join(filepath.Base(path), relativePath)

		f := filepath.Join(root, relativePath)
		_, err := os.Stat(f)
		if !os.IsNotExist(err) {
			return relativePath
		}
		path = filepath.Dir(path)
	}
	return ""
}

func replaceFileName(profiles []*profile.Profile, root string) error {
	for _, p := range profiles {
		file := relativePathFromRoot(p.FileName, root)
		if file == "" {
			return fmt.Errorf("file not found: %s", p.FileName)
		}
		log.Print("file=", file)
		p.FileName = file
	}
	return nil
}

func parseFile(filename, root, entryName string) (*server.CoverageEntryUploadRequest, error) {
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

	e := &server.CoverageEntryUploadRequest{
		EntryName: entryName,
		Profiles:  profiles,
		Hits:      hits,
		Lines:     lines,
	}

	return e, nil
}

func upload(serverURL string, req *server.CoverageUploadRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	url := serverURL + "/api/upload"
	r, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	client := http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return err
	}

	msg, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		log.Print(msg)
		return errors.New("returned status is not StatusOK")
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

func checkRequest(req *server.CoverageUploadRequest, repo *git.Repository) (bool, error) {
	isDirty, err := isDirty(repo)
	if err != nil {
		return false, err
	}

	return !isDirty, nil
}

func makeRequest(repo *git.Repository, url, entryName string, files ...string) (*server.CoverageUploadRequest, error) {
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
	root := wt.Filesystem.Root()

	entries := []*server.CoverageEntryUploadRequest{}
	for _, file := range files {
		e, err := parseFile(file, root, entryName)
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

	req := &server.CoverageUploadRequest{
		RepoURL:  url,
		Revision: commit.Hash.String(),
		Time:     commit.Committer.When,
		Entries:  entries,
	}

	return req, nil
}

func Upload(server, repoURL, repoPath, entryName string, dryRun, force bool, args []string) error {

	log.Print("force=", force)
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
	log.Print(flag)
	if (!force) && !flag {
		fmt.Println("working tree is dirty")
		return err
	}

	fmt.Println("Revision:", req.Revision)
	fmt.Println("Time:", req.Time)

	if !dryRun {
		if server == "" {
			fmt.Println("use -server=<server url>")
			os.Exit(1)
		}

		err = upload(server, req)
		if err != nil {
			return err
		}

		fmt.Println("Uploaded")
	}

	return nil
}
