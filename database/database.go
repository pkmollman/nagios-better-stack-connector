package database

import (
	"database/sql"

	"github.com/pkmollman/nagios-better-stack-connector/models"
)

type DatabaseClient interface {
	Init(db *sql.DB) error
	// should be safe to call multiple times
	CreateEventItemTable() error
	CreateEventItem(item models.EventItem) error
	GetAllEventItems() ([]models.EventItem, error)
	Lock()
	Unlock()
	Shutdown() error
}
