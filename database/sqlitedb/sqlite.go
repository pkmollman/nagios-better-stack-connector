package sqlitedb

import (
	"database/sql"

	"github.com/pkmollman/nagios-better-stack-connector/database"
	"github.com/pkmollman/nagios-better-stack-connector/models"
	_ "modernc.org/sqlite"
)

type SQLiteClient struct {
	db         *sql.DB
	serialChan chan struct{}
}

func NewSQLiteClient(db_path string) (database.DatabaseClient, error) {
	// sqlite
	db, err := sql.Open("sqlite", db_path)
	if err != nil {
		return nil, err
	}
	return &SQLiteClient{
		db: db,
	}, nil
}

func (s *SQLiteClient) Lock() {
	s.serialChan <- struct{}{}
}

func (s *SQLiteClient) Unlock() {
	<-s.serialChan
}

func (s *SQLiteClient) Init() error {
	// only one operation at a time
	s.serialChan = make(chan struct{}, 1)
	s.Lock()
	defer s.Unlock()
	err := s.CreateEventItemTable()
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) Shutdown() error {
	err := s.db.Close()
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) CreateEventItemTable() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY,
		nagiosSiteName TEXT,
		nagiosProblemId TEXT,
		nagiosProblemType TEXT,
		nagiosProblemHostname TEXT,
		nagiosProblemServiceName TEXT,
		nagiosProblemContent TEXT,
		nagiosProblemNotificationType TEXT,
		betterStackPolicyId TEXT,
		betterStackIncidentId TEXT )`)

	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) CreateEventItem(item models.EventItem) error {
	// insert into database
	insetStmt, err := s.db.Prepare(`
	INSERT INTO events (
		nagiosSiteName,
		nagiosProblemId,
		nagiosProblemType,
		nagiosProblemHostname,
		nagiosProblemServiceName,
		nagiosProblemContent,
		nagiosProblemNotificationType,
		betterStackPolicyId,
		betterStackIncidentId )
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	_, err = insetStmt.Exec(
		item.NagiosSiteName,
		item.NagiosProblemId,
		item.NagiosProblemType,
		item.NagiosProblemHostname,
		item.NagiosProblemServiceName,
		item.NagiosProblemContent,
		item.NagiosProblemNotificationType,
		item.BetterStackPolicyId,
		item.BetterStackIncidentId,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) GetAllEventItems() ([]models.EventItem, error) {
	stmt, err := s.db.Prepare(`
	SELECT
		id,
		nagiosSiteName,
		nagiosProblemId,
		nagiosProblemType,
		nagiosProblemHostname,
		nagiosProblemServiceName,
		nagiosProblemContent,
		nagiosProblemNotificationType,
		betterStackPolicyId,
		betterStackIncidentId
	FROM events
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.EventItem{}
	for rows.Next() {
		var item models.EventItem
		err := rows.Scan(
			&item.Id,
			&item.NagiosSiteName,
			&item.NagiosProblemId,
			&item.NagiosProblemType,
			&item.NagiosProblemHostname,
			&item.NagiosProblemServiceName,
			&item.NagiosProblemContent,
			&item.NagiosProblemNotificationType,
			&item.BetterStackPolicyId,
			&item.BetterStackIncidentId,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
