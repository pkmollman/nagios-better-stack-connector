package database

import (
	"github.com/pkmollman/nagios-better-stack-connector/models"
)

type DatabaseClient interface {
	Init() error
	// should be safe to call multiple times
	CreateEventItemTable() error
	CreateEventItem(item models.EventItem) error
	GetAllEventItems() ([]models.EventItem, error)
	Lock()
	Unlock()
	Shutdown() error
}
