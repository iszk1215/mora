//go:build !oss

package mockscm

//go:generate mockgen -package=mockscm -destination=mock_gen.go github.com/drone/go-scm/scm ContentService,RepositoryService
