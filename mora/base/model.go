package base

import (
	"context"

	"github.com/drone/go-scm/scm"
)

type (
	RepositoryClient interface {
		Client() *scm.Client
		RevisionURL(baseURL string, revision string) string
	}

	Repository struct {
		Id                int64  `json:"id"`
		RepositoryManager int64  `json:"scm_id"`
		Namespace         string `json:"namespace"`
		Name              string `json:"name"`
		Url               string `json:"url"`
	}
)

type (
	contextKey int
)

const (
	contextRepoKey              contextKey = iota
	contextRepositoryClientKey  contextKey = iota
)

func WithRepositoryClient(ctx context.Context, client RepositoryClient) context.Context {
	return context.WithValue(ctx, contextRepositoryClientKey, client)
}

func RepositoryClientFrom(ctx context.Context) (RepositoryClient, bool) {
	rm, ok := ctx.Value(contextRepositoryClientKey).(RepositoryClient)
	return rm, ok
}

func WithRepo(ctx context.Context, repo Repository) context.Context {
	return context.WithValue(ctx, contextRepoKey, repo)
}

func RepoFrom(ctx context.Context) (Repository, bool) {
	repo, ok := ctx.Value(contextRepoKey).(Repository)
	return repo, ok
}
