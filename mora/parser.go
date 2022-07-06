package mora

import (
	"bufio"
	"errors"
	"io"
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

func postprocess(profiles []*Profile, prefix string) {
	for _, p := range profiles {
		p.FileName = strings.Replace(p.FileName, prefix, "", -1)

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

func convertLcovToGcov(reader io.Reader, prefix string) ([]*Profile, error) {
	scanner := bufio.NewScanner(reader)

	profiles := []*Profile{}

	filename := ""
	var block []int = nil
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
			if block != nil && block[END]+1 == start && block[COUNT] == count {
				block[END] = start
			} else {
				block = []int{start, start, count}
				blocks = append(blocks, block)
			}
		case "end_of_record":
			if filename == "" {
				return nil, errors.New("no SF found for this TN")
			}
			prof := &Profile{FileName: filename, Blocks: blocks}
			profiles = append(profiles, prof)

			filename = ""
			block = nil
			blocks = nil
		}
	}

	return profiles, nil
}

func ParseLcov(reader io.Reader, prefix string) ([]*Profile, error) {
	profiles, err := convertLcovToGcov(reader, prefix)
	if err != nil {
		return nil, err
	}
	postprocess(profiles, prefix)
	return profiles, nil
}

func convertGoProfile(profile *cover.Profile, moduleName string) *Profile {
	file := strings.Replace(profile.FileName, moduleName, "", -1)

	pr := &Profile{
		FileName: file, Hits: 0, Lines: 0, Blocks: [][]int{}}

	for _, b := range profile.Blocks {
		pr.Blocks = append(pr.Blocks, []int{b.StartLine, b.EndLine, b.Count})
		pr.Lines += b.NumStmt
		if b.Count > 0 {
			pr.Hits += b.NumStmt
		}
	}

	return pr
}

func ParseGoCover(reader io.Reader, moduleName string) ([]*Profile, error) {
	goProfiles, err := cover.ParseProfilesFromReader(reader)
	if err != nil {
		return nil, err
	}

	profiles := []*Profile{}
	for _, profile := range goProfiles {
		pr := convertGoProfile(profile, moduleName)
		profiles = append(profiles, pr)
	}

	return profiles, nil
}

func ParseToolCoverage(reader io.Reader, format, prefix string) ([]*Profile, error) {
	switch format {
	case "lcov":
		return ParseLcov(reader, prefix)
	case "go":
		return ParseGoCover(reader, prefix)
	}

	return nil, errors.New("unknown coverage format")
}
