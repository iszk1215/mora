package model

import "context"

type (
	Repository struct {
		Id        int64  `json:"id"`
		SCM       int64  `json:"scm_id"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Url       string `json:"url"`
	}
)

type (
    contextKey int
)

const (
	contextRepoKey contextKey = iota
	ContextSCMKey  contextKey = iota
)

func WithRepo(ctx context.Context, repo Repository) context.Context {
	return context.WithValue(ctx, contextRepoKey, repo)
}

func RepoFrom(ctx context.Context) (Repository, bool) {
	repo, ok := ctx.Value(contextRepoKey).(Repository)
	return repo, ok
}

