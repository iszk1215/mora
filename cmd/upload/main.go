package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/iszk1215/mora/mora"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func ParseToolCoverageFromFile(filename, format, prefix string) ([]*mora.Profile, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return mora.ParseCoverage(reader, format, prefix)
}

func parseFromSpec(spec string) (*mora.CoverageEntryUploadRequest, error) {
	tmp := strings.Split(spec, ",")
	if len(tmp) != 4 {
		err := errors.New("format error")
		return nil, err
	}
	format := tmp[0]
	entry := tmp[1]
	prefix := tmp[2]
	filename := tmp[3]

	profiles, err := ParseToolCoverageFromFile(filename, format, prefix)
	if err != nil {
		log.Err(err).Msg("parse error: ")
		os.Exit(1)
	}

	hits := 0
	lines := 0
	for _, p := range profiles {
		hits += p.Hits
		lines += p.Lines
	}

	e := &mora.CoverageEntryUploadRequest{
		EntryName: entry,
		Profiles:  profiles,
		Hits:      hits,
		Lines:     lines,
	}

	return e, nil
}

func upload(serverURL string, req *mora.CoverageUploadRequest) error {
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

func checkRequest(req *mora.CoverageUploadRequest, repo *git.Repository) (bool, error) {
	isDirty, err := isDirty(repo)
	if err != nil {
		return false, err
	}

	return !isDirty, nil
}

func makeRequest(repo *git.Repository, url string, specs ...string) (*mora.CoverageUploadRequest, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	entries := []*mora.CoverageEntryUploadRequest{}
	for _, spec := range specs {
		e, err := parseFromSpec(spec)
		if err != nil {
			log.Err(err).Msg("")
			os.Exit(1)
		}

		entries = append(entries, e)
	}

	req := &mora.CoverageUploadRequest{
		RepoURL:  url,
		Revision: commit.Hash.String(),
		Time:     commit.Committer.When,
		Entries:  entries,
	}

	return req, nil
}

func main() {
	log.Logger = zerolog.New(os.Stderr).With().Caller().Logger()

	server := flag.String("server", "", "server")
	repoURL := flag.String("repo", "", "URL of repository")
	repoPath := flag.String("repo-path", "", "path of repository")
	force := flag.Bool("f", false, "force upload even when working tree is dirty")
	dryRun := flag.Bool("dry-run", false, "dry run")

	flag.Parse()
	args := flag.Args()

	if *server == "" {
		fmt.Println("use -server=<server url>")
		os.Exit(1)
	}

	repo, err := git.PlainOpen(*repoPath)
	if err != nil {
		fmt.Println("Can not open repository. Use -repo-path=<repository>")
		os.Exit(1)
	}

	req, err := makeRequest(repo, *repoURL, args...)
	if err != nil {
		log.Fatal().Msg("failed to make a request")
	}

	flag, err := checkRequest(req, repo)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if (!*force) && !flag {
		fmt.Println("working tree is dirty")
		os.Exit(1)
	}

	fmt.Println("Revision:", req.Revision)
	fmt.Println("Time:", req.Time)

	if !*dryRun {
		err = upload(*server, req)
		if err != nil {
			log.Err(err).Msg("upload")
			os.Exit(1)
		}

		fmt.Println("Uploaded")
	}
}
