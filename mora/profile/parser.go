package profile

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/cover"
)

type Profile struct {
	FileName string  `json:"filename"`
	Hits     int     `json:"hits"`
	Lines    int     `json:"lines"`
	Blocks   [][]int `json:"blocks"` // StartLine, EndLine, Count
}

const (
	START int = iota
	END   int = iota
	COUNT int = iota
)

func mergeBlocks(blocks [][]int) [][]int {
	if len(blocks) < 2 {
		return blocks
	}

	block := blocks[0]
	ret := [][]int{block}
	for _, b := range blocks[1:] {
		if block[END]+1 == b[START] && block[COUNT] == b[COUNT] {
			block[END] = b[END]
		} else {
			block = b
			ret = append(ret, block)
		}
	}
	return ret
}

func postprocess(profiles []*Profile) {
	for _, p := range profiles {
		p.Blocks = mergeBlocks(p.Blocks)

		p.Hits = 0
		p.Lines = 0
		for _, b := range p.Blocks {
			l := b[END] - b[START] + 1
			if b[COUNT] > 0 {
				p.Hits += l
			}
			p.Lines += l
		}
	}
}

func parseLcov(reader io.Reader) ([]*Profile, error) {
	scanner := bufio.NewScanner(reader)

	profiles := []*Profile{}

	filename := ""
	var blocks [][]int = nil

	for scanner.Scan() {
		line := scanner.Text()
		list := strings.Split(line, ":")
		switch list[0] {
		case "TN":
			blocks = [][]int{}
		case "SF":
			filename = list[1]
		case "DA":
			tmp := strings.Split(list[1], ",")
			start, err := strconv.Atoi(tmp[0])
			if err != nil {
				return nil, err
			}
			count, err := strconv.Atoi(tmp[1])
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, []int{start, start, count})
		case "end_of_record":
			if filename == "" {
				return nil, errors.New("no SF found for this TN")
			}
			prof := &Profile{FileName: filename, Blocks: blocks}
			profiles = append(profiles, prof)

			filename = ""
			blocks = nil
		}
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profile found")
	}

	return profiles, nil
}

func convertGoProfile(profile *cover.Profile) *Profile {
	pr := &Profile{FileName: profile.FileName}

	// Change block to lines.
	//
	// NOTE: We do not use NumStmt because it is difficult to use without
	// source code.
	//
	// Example:
	//   10  if (condition) {
	//   11    // do something
	//   12  }
	// For this code block, we have a cover.ProfileBlock with StartLine=10,
	// EndLine=12, NumStmt=1. Three lines, 10, 11 and 12 are created here from
	// this block regardless of NumStmt=1
	blocks := [][]int{}
	for _, b := range profile.Blocks {
		for l := b.StartLine; l <= b.EndLine; l++ {
			blocks = append(blocks, []int{l, l, b.Count})
		}
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i][START] < blocks[j][START]
	})

	var block []int = nil
	list := [][]int{}
	for _, b := range blocks {
		if block == nil {
			block = b
			list = append(list, block)
		} else if block[START] == b[START] {
			block[COUNT] += b[COUNT]
		} else {
			block = b
			list = append(list, block)
		}
	}

	pr.Blocks = list
	return pr
}

func parseGocov(reader io.Reader) ([]*Profile, error) {
	goProfiles, err := cover.ParseProfilesFromReader(reader)
	if err != nil {
		return nil, err
	}

	profiles := []*Profile{}
	for _, profile := range goProfiles {
		pr := convertGoProfile(profile)
		profiles = append(profiles, pr)
	}

	return profiles, nil
}

func ParseCoverage(reader io.Reader) ([]*Profile, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	profiles, err := parseLcov(bytes.NewReader(b))
	if err != nil {
		profiles, err = parseGocov(bytes.NewReader(b))
	}

	if err != nil {
		return nil, err
	}

	postprocess(profiles)
	return profiles, nil
}
