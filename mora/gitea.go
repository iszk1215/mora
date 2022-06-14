package mora

import (
	login "github.com/drone/go-login/login/gitea"
	driver "github.com/drone/go-scm/scm/driver/gitea"
)

type Gitea struct {
	BaseSCM
}

func NewGitea(name string, url string, config login.Config) (*Gitea, error) {
	client, err := driver.New(url)
	if err != nil {
		return nil, err
	}

	gitea := new(Gitea)
	gitea.Init(name, client.BaseURL, client, &config)

	return gitea, nil
}

func (g *Gitea) RevisionURL(repo *Repo, revision string) string {
	return repo.Link + "/src/commit/" + revision
}

func NewGiteaFromFile(name string, filename string, url string, redirect_url string) (*Gitea, error) {
	secret, err := readSecret(filename)
	if err != nil {
		return nil, err
	}

	config := login.Config{
		ClientID:     secret.ClientID,
		ClientSecret: secret.ClientSecret,
		Server:       url,
		RedirectURL:  redirect_url,
	}

	return NewGitea(name, url, config)
}
