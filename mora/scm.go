package mora

import (
	"context"
	"net/http"
	"net/url"
	"os"

	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/transport/oauth2"
	"gopkg.in/yaml.v3"
)

type BaseSCM struct {
	name   string
	client *scm.Client
	url    *url.URL
}

func (s *BaseSCM) Init(name string, client *scm.Client, url *url.URL) {
	s.name = name
	s.url = url
	s.client = client
	s.client.Client = &http.Client{
		Transport: &oauth2.Transport{
			Scheme: "token",
			Source: oauth2.ContextTokenSource(),
		},
	}
}

func (s *BaseSCM) Name() string {
	return s.name
}

func (s *BaseSCM) URL() *url.URL {
	return s.url
}

func (s *BaseSCM) GetRepos(token *scm.Token) ([]*Repo, error) {
	ctx := scm.WithContext(context.Background(), token)

	ret := []*Repo{}
	opts := scm.ListOptions{Size: 100}
	for {
		result, meta, err := s.client.Repositories.List(ctx, opts)
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
