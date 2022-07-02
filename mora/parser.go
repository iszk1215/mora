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
	Blocks   [][]int `json:"blocks"`
}

func split(text string, sep byte) (string, string) {
	idx := strings.IndexByte(text, sep)
	if idx < 0 {
		return "", text
	}
	return text[:idx], text[idx+1:]
}

func ParseLcov(reader io.Reader, prefix string) ([]*Profile, error) {
	scanner := bufio.NewScanner(reader)

	var prof *cover.Profile = nil
	profiles := []*cover.Profile{}

	var block *cover.ProfileBlock = nil

	for scanner.Scan() {
		line := scanner.Text()
		typ, value := split(line, ':')
		switch typ {
		case "SF":
			// log.Print("text=", value)
			if block != nil {
				prof.Blocks = append(prof.Blocks, *block)
			}
			prof = &cover.Profile{FileName: value, Blocks: []cover.ProfileBlock{}}
			profiles = append(profiles, prof)
			block = nil
		case "DA":
			// log.Print("DA=", value)
			a, b := split(value, ',')
			start, err := strconv.Atoi(a)
			if err != nil {
				return nil, err
			}
			count, err := strconv.Atoi(b)
			if err != nil {
				return nil, err
			}
			if block != nil && block.EndLine+1 == start && block.Count == count {
				block.EndLine = start
				block.NumStmt += 1
			} else {
				if block != nil {
					prof.Blocks = append(prof.Blocks, *block)
				}
				block = &cover.ProfileBlock{}
				block.StartLine = start
				block.StartCol = 0
				block.EndLine = start
				block.EndCol = 0
				block.NumStmt = 1
				block.Count = count
			}
		}
	}

	return convertGoProfiles(profiles, prefix)
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

func convertGoProfiles(goProfiles []*cover.Profile, moduleName string) ([]*Profile, error) {
	profiles := []*Profile{}
	for _, profile := range goProfiles {
		pr := convertGoProfile(profile, moduleName)
		profiles = append(profiles, pr)
	}

	return profiles, nil
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

func ParseToolCoverage(reader io.Reader, format, moduleName string) ([]*Profile, error) {

	switch format {
	case "lcov":
		return ParseLcov(reader, moduleName)
	case "go":
		return ParseGoCover(reader, moduleName)
	}

	return nil, errors.New("unknown coverage format")
}
