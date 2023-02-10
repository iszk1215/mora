package server

import (
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

var schema_repo = `
CREATE TABLE IF NOT EXISTS repository (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scm INTEGER NOT NULL,
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    UNIQUE(scm, namespace, name)
)`

type (
	storableRepository struct {
		ID        int64  `db:"id"`
		SCM       int64  `db:"scm"`
		Namespace string `db:"namespace"`
		Name      string `db:"name"`
		URL       string `db:"url"`
	}

	repositoryStoreImpl struct {
		db *sqlx.DB
	}
)

func NewRepositoryStore(db *sqlx.DB) RepositoryStore {
	return &repositoryStoreImpl{db}
}

func (s *repositoryStoreImpl) Init() error {
	_, err := s.db.Exec(schema_repo)
	if err != nil {
		log.Err(err).Msg("")
		return err
	}
	return nil
}

func toRepo(from storableRepository) Repository {
	return Repository{
		ID:        from.ID,
		SCM:       from.SCM,
		Namespace: from.Namespace,
		Name:      from.Name,
		Link:      from.URL,
	}
}

func (s *repositoryStoreImpl) findOne(query string, params ...interface{}) (Repository, error) {
	rows := []storableRepository{}
	err := s.db.Select(&rows, query, params...)
	if err != nil {
		return Repository{}, err
	}

	if len(rows) == 0 {
		return Repository{}, errors.New("no repo")
	}

	return toRepo(rows[0]), nil
}

func (s *repositoryStoreImpl) Find(id int64) (Repository, error) {
	query := "SELECT id, scm, namespace, name, url FROM repository WHERE id = ?"
	return s.findOne(query, id)
}

func (s *repositoryStoreImpl) FindURL(url string) (Repository, error) {
	query := "SELECT id, scm, namespace, name, url FROM repository WHERE url = ?"
	return s.findOne(query, url)
}

func (s *repositoryStoreImpl) Put(repo *Repository) error {
	res, err := s.db.Exec(
		"INSERT INTO repository (scm, namespace, name, url) VALUES ($1, $2, $3, $4)",
		repo.SCM, repo.Namespace, repo.Name, repo.Link)
	if err != nil {
		return err
	}

	repo.ID, err = res.LastInsertId()
	return err
}

func (s *repositoryStoreImpl) Scan() ([]Repository, error) {
	rows := []storableRepository{}
	err := s.db.Select(&rows, "SELECT id, scm, name, namespace, url FROM repository")

	if err != nil {
		return nil, err
	}

	repos := []Repository{}
	for _, record := range rows {
		repo := Repository{
			ID:        record.ID,
			SCM:       record.SCM,
			Namespace: record.Namespace,
			Name:      record.Name,
			Link:      record.URL,
		}
		repos = append(repos, repo)
	}

	return repos, nil
}
