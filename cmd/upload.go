/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/iszk1215/mora/mora"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func ParseCoverageFromFile(filename string) ([]*mora.Profile, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return mora.ParseCoverage(reader)
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

func replaceFileName(profiles []*mora.Profile, root string) error {
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

func parseFile(filename, root, entryName string) (*mora.CoverageEntryUploadRequest, error) {
	profiles, err := ParseCoverageFromFile(filename)
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

	e := &mora.CoverageEntryUploadRequest{
		EntryName: entryName,
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

func makeRequest(repo *git.Repository, url, entryName string, files ...string) (*mora.CoverageUploadRequest, error) {
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

	entries := []*mora.CoverageEntryUploadRequest{}
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

	req := &mora.CoverageUploadRequest{
		RepoURL:  url,
		Revision: commit.Hash.String(),
		Time:     commit.Committer.When,
		Entries:  entries,
	}

	return req, nil
}

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		noColor := false
		o, _ := os.Stderr.Stat()
		if (o.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
			noColor = true
		}

		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: noColor}).With().Caller().Logger()

		server, _ := cmd.Flags().GetString("server")
		repoURL, _ := cmd.Flags().GetString("repo")
		repoPath, _ := cmd.Flags().GetString("repo-path")
		force, _ := cmd.Flags().GetBool("force")
		entryName, _ := cmd.Flags().GetString("entry")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

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
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// uploadCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// uploadCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	uploadCmd.Flags().String("server", "", "server url")
	uploadCmd.Flags().String("repo-path", "", "path of repositry")
	uploadCmd.Flags().String("repo", "", "URL")
	uploadCmd.Flags().String("entry", "_default", "entry name")
	uploadCmd.Flags().BoolP("force", "f", false, "force upload even when working tree is dirty")
	uploadCmd.Flags().Bool("dry-run", false, "test")
}
