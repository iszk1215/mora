package udm

import (
	"errors"
	"sync"

	"github.com/jmoiron/sqlx"
)

var (
	errorMetricInUse    = errors.New("metric in use")
	errorMetricNotFound = errors.New("no metric found")
	errorItemInUse      = errors.New("item in use")
	errorItemNotFound   = errors.New("no item found")
)

var schema_metric = `
CREATE TABLE IF NOT EXISTS udm_metric (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL CHECK(repo_id > 0),
    name TEXT NOT NULL,
    UNIQUE(repo_id, name)
)`

var schema_item = `
CREATE TABLE IF NOT EXISTS udm_item (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    type INTEGER NOT NULL,
    UNIQUE(metric_id, name)
)`

var schema_value = `
CREATE TABLE IF NOT EXISTS udm_value (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    value TEXT NOT NULL,
    UNIQUE(item_id, time)
)`

type (
	udmStore struct {
		db *sqlx.DB
		sync.Mutex
	}

	MetricType int
)

func newUdmStore(db *sqlx.DB) *udmStore {
	return &udmStore{db: db}
}

// ----------------------------------------------------------------------
// Metric

func (s *udmStore) addMetric(metric *metricModel) error {
	query := "INSERT INTO udm_metric (repo_id, name) VALUES ($1, $2)"

	res, err := s.db.Exec(query, metric.RepoId, metric.Name)
	if err != nil {
		return err
	}

	metric.Id, err = res.LastInsertId()
	if err != nil {
		return err
	}

	return nil
}

func (s *udmStore) listMetrics(repoId int64) ([]metricModel, error) {
	query := "SELECT id,repo_id,name FROM udm_metric WHERE repo_id = ?"

	rows := []metricModel{}
	err := s.db.Select(&rows, query, repoId)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (s *udmStore) findMetricById(id int64) (*metricModel, error) {
	query := "SELECT id,repo_id,name FROM udm_metric WHERE id = ?"

	rows := []*metricModel{}
	err := s.db.Select(&rows, query, id)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, errorMetricNotFound
	}

	return rows[0], nil
}

func (s *udmStore) deleteMetric(id int64) error {
	items, err := s.listItems(id)
	if err != nil {
		return err
	}

	if len(items) != 0 {
		return errorMetricInUse
	}

	query := "DELETE FROM udm_metric WHERE id = $1"
	_, err = s.db.Exec(query, id)
	return err
}

// ----------------------------------------------------------------------
// Item

func (s *udmStore) addItem(item *itemModel) error {
	// ensure that metric exists
	_, err := s.findMetricById(item.MetricId)
	if err != nil {
		return err
	}

	query := "INSERT INTO udm_item (metric_id, name, type) VALUES ($1, $2, $3)"

	res, err := s.db.Exec(query, item.MetricId, item.Name, item.ValueType)
	if err != nil {
		return err
	}

	item.Id, err = res.LastInsertId()
	if err != nil {
		return err
	}

	return nil
}

func (s *udmStore) deleteItem(id int64) error {
	values, err := s.listValues(id)
	if err != nil {
		return err
	}

	if len(values) > 0 {
		return errorItemInUse
	}

	query := "DELETE FROM udm_item WHERE id = $1"

	_, err = s.db.Exec(query, id)
	return err
}

func (s *udmStore) findItemById(id int64) (*itemModel, error) {
	query := "SELECT id,metric_id,name,type FROM udm_item WHERE id = ?"

	rows := []itemModel{}
	err := s.db.Select(&rows, query, id)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, errorItemNotFound
	}

	return &rows[0], nil
}

func (s *udmStore) listItems(metricId int64) ([]itemModel, error) {
	query := "SELECT id,metric_id,name,type FROM udm_item WHERE metric_id = ?"

	rows := []itemModel{}
	err := s.db.Select(&rows, query, metricId)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

// ----------------------------------------------------------------------
// Value

func (s *udmStore) addValue(value *valueModel) error {
	_, err := s.findItemById(value.ItemId)
	if err != nil {
		return err
	}

	query := "INSERT INTO udm_value (item_id, revision, time, value) VALUES ($1, $2, $3, $4)"

	res, err := s.db.Exec(query, value.ItemId, value.Revision, value.Timestamp, value.Value)
	if err != nil {
		return err
	}

	value.Id, err = res.LastInsertId()
	if err != nil {
		return err
	}

	return nil
}

func (s *udmStore) listValues(itemId int64) ([]valueModel, error) {
	query := "SELECT id,item_id,revision,time,value FROM udm_value WHERE item_id = ? ORDER BY time"

	rows := []valueModel{}
	err := s.db.Select(&rows, query, itemId)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (s *udmStore) deleteValues(itemId int64) error {
	query := "DELETE FROM udm_value WHERE item_id = $1"
	_, err := s.db.Exec(query, itemId)
	return err
}

func (s *udmStore) initialize() error {
	_, err := s.db.Exec(schema_metric)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(schema_item)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(schema_value)
	if err != nil {
		return err
	}

	return err
}
