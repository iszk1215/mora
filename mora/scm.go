package mora

import (
	"context"
	"os"

	"github.com/drone/go-scm/scm"
	"gopkg.in/yaml.v3"
)

func getRepos(client *scm.Client, name string, token *scm.Token) ([]*Repo, error) {
	ctx := scm.WithContext(context.Background(), token)

	ret := []*Repo{}
	opts := scm.ListOptions{Size: 100}
	for {
		result, meta, err := client.Repositories.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		ret = append(ret, result...)

		opts.Page = meta.Page.Next
		opts.URL = meta.Page.NextURL

		if opts.Page == 0 && opts.URL == "" {
			break
		}
	}
	return ret, nil
}

type secret struct {
	ClientID     string `yaml:"ClientID"`
	ClientSecret string `yaml:"ClientSecret"`
}

func readSecret(filename string) (secret, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return secret{}, err
	}

	s := secret{}
	err = yaml.Unmarshal(b, &s)
	if err != nil {
		return secret{}, err
	}

	return s, nil
}
