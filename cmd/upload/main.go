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
	"time"

	"github.com/iszk1215/mora/mora"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func ParseToolCoverageFromFile(filename, format, prefix string) ([]*mora.Profile, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return mora.ParseToolCoverage(reader, format, prefix)
}

func upload(serverURL string, profiles []*mora.Profile, req mora.CoverageUploadRequest) error {
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

func main() {
	log.Logger = zerolog.New(os.Stderr).With().Caller().Logger()

	revision := flag.String("revision", "", "revision")
	format := flag.String("format", "", "format")
	entry := flag.String("entry", "", "coverage entry")
	repoURL := flag.String("repo", "", "URL of repository")
	prefix := flag.String("prefix", "",
		"remove prefix from filename to get relative path from repository root")
	timestampString := flag.String("time", "", "timestamp in RFC3339 format")

	flag.Parse()
	args := flag.Args()

	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: upload [options] file serverURL\n")
		os.Exit(1)
	}
	filename := args[0]
	moraServerURL := args[1]

	timestamp, err := time.Parse(time.RFC3339, *timestampString)
	if err != nil {
		log.Err(err).Msg("")
		os.Exit(1)
	}

	profiles, err := ParseToolCoverageFromFile(filename, *format, *prefix)
	if err != nil {
		log.Err(err).Msg("parse error: ")
		os.Exit(1)
	}

	req := mora.CoverageUploadRequest{
		Format:     *format,
		EntryName:  *entry,
		RepoURL:    *repoURL,
		Revision:   *revision,
		ModuleName: *prefix,
		Time:       timestamp,
		Profiles:   profiles,
	}

	err = upload(moraServerURL, profiles, req)
	if err != nil {
		log.Err(err).Msg("upload")
		os.Exit(1)
	}
}
