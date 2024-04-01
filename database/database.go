package database

import (
	"github.com/pkmollman/nagios-better-stack-connector/models"
)

type DatabaseClient interface {
	Init() error
	// should be safe to call multiple times
	CreateEventItemTable() error
	CreateEventItem(item models.EventItem) (int64, error)
	DeleteEventItem(id int64) (int64, error)
	GetAllEventItems() ([]models.EventItem, error)
	Lock()
	Unlock()
	Shutdown() error
	Backup()
}
