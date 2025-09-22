package sqlitedb

import (
	"database/sql"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/pkmollman/nagios-better-stack-connector/database"
	"github.com/pkmollman/nagios-better-stack-connector/models"
	_ "modernc.org/sqlite"
)

type SQLiteClient struct {
	db              *sql.DB
	serialChan      chan struct{}
	backupDirectory string
}

func NewSQLiteClient(db_path, backup_directory string) (database.DatabaseClient, error) {
	// sqlite
	db, err := sql.Open("sqlite", db_path)
	if err != nil {
		return nil, err
	}

	client := SQLiteClient{
		db:              db,
		backupDirectory: backup_directory,
		serialChan:      make(chan struct{}, 1),
	}

	return &client, nil
}

func (s *SQLiteClient) Backup() {
	// generate timestamp
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	filestring := s.backupDirectory + "/backup-" + timestamp + ".db"
	s.db.Exec(`VACUUM INTO "` + filestring + `"`)
}

func (s *SQLiteClient) Lock() {
	s.serialChan <- struct{}{}
	fmt.Println("GOT DB LOCK")
	fmt.Println(string(debug.Stack()))
}

func (s *SQLiteClient) Unlock() {
	<-s.serialChan
	fmt.Println("DB UNLOCKED")
	fmt.Println(string(debug.Stack()))
}

func (s *SQLiteClient) Init() error {
	// only one operation at a time
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

func (s *SQLiteClient) CreateEventItem(item models.EventItem) (int64, error) {
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
		return 0, err
	}
	result, err := insetStmt.Exec(
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
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (s *SQLiteClient) DeleteEventItem(id int64) (int64, error) {
	stmt, err := s.db.Prepare("DELETE FROM events WHERE id = ?")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(id)
	if err != nil {
		return 0, err
	}

	rowsEffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsEffected, nil
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
