package web

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"text/template"

	"github.com/pkmollman/nagios-better-stack-connector/models"
)

const (
	HEALTHY   = "HEALTHY"
	UNHEALTHY = "UNHEALTHY"
)

type nbscServiceCheckState struct {
	Succeeded bool
	Message   string
}

type nbscServiceStatus struct {
	State       string
	CheckStates []nbscServiceCheckState
}

func newNbscServiceStatus() nbscServiceStatus {
	return nbscServiceStatus{
		State:       HEALTHY,
		CheckStates: []nbscServiceCheckState{},
	}
}

func (nss *nbscServiceStatus) NewFailure(message string) {
	nss.State = UNHEALTHY
	nss.CheckStates = append(nss.CheckStates, nbscServiceCheckState{Succeeded: false, Message: message})
}

func (nss *nbscServiceStatus) NewSuccess(message string) {
	nss.CheckStates = append(nss.CheckStates, nbscServiceCheckState{Succeeded: true, Message: message})
}

type nbscStatus struct {
	Database    nbscServiceStatus
	Nagios      nbscServiceStatus
	BetterStack nbscServiceStatus
}

func (wh *WebHandler) handleHealthRequest(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.Header().Set("Content-Type", "text/plain")

	connectorStatus := nbscStatus{
		Database:    newNbscServiceStatus(),
		Nagios:      newNbscServiceStatus(),
		BetterStack: newNbscServiceStatus(),
	}

	const healthTextTemplate = `Database: {{.Database.State}}
{{- range .Database.CheckStates}}
{{if .Succeeded}}  - SUCCESS: {{else}}  - FAILURE: {{end}}{{.Message}}
{{- end}}

Nagios: {{.Nagios.State}}
{{- range .Nagios.CheckStates}}
{{if .Succeeded}}  - SUCCESS: {{else}}  - FAILURE: {{end}}{{.Message}}
{{- end}}

BetterStack: {{.BetterStack.State}}
{{- range .BetterStack.CheckStates}}
{{if .Succeeded}}  - SUCCESS: {{else}}  - FAILURE: {{end}}{{.Message}}
{{- end}}
`

	// check database
	wh.dbClient.Lock()
	_, err := wh.dbClient.GetAllEventItems()
	if err != nil {
		connectorStatus.Database.NewFailure("Failed to get event items from database: " + err.Error())
	} else {
		connectorStatus.Database.NewSuccess("Successfully got event items from database")
	}

	newId, err := wh.dbClient.CreateEventItem(models.EventItem{})
	if err != nil {
		connectorStatus.Database.NewFailure("Failed to create event item in database: " + err.Error())
	} else {
		connectorStatus.Database.NewSuccess("Successfully created event item in database")
	}

	rowsEffected, err := wh.dbClient.DeleteEventItem(newId)
	if err != nil {
		connectorStatus.Database.NewFailure("Failed to delete event item in database: " + err.Error())
	} else {
		connectorStatus.Database.NewSuccess("Successfully attempted to delete event item in database")
	}

	if rowsEffected != 1 {
		connectorStatus.Database.NewFailure("Failed to delete event item in database: expected 1 row affected, got " + fmt.Sprint(rowsEffected))
	} else {
		connectorStatus.Database.NewSuccess("Successfully deleted event item in database")
	}

	wh.dbClient.Unlock()

	// check nagios
	hosts, err := wh.nagiosClient.GetHosts()
	if err != nil {
		connectorStatus.Nagios.NewFailure("Failed to get hosts from Nagios: " + err.Error())
	} else {
		connectorStatus.Nagios.NewSuccess("Successfully got hosts from Nagios")
	}

	// pick a random host
	if err == nil && len(hosts) > 0 {
		host := hosts[rand.Intn(len(hosts))]

		for len(host.Services) == 0 {
			host = hosts[rand.Intn(len(hosts))]
		}

		// get random sercice name
		serviceName := host.Services[rand.Intn(len(host.Services))]

		// check service
		service, err := wh.nagiosClient.GetServiceState(host.DisplayName, serviceName)
		if err != nil {
			connectorStatus.Nagios.NewFailure(
				fmt.Sprintf(
					`Failed to get Nagios service state for HOST="%s" SERVICE="%s": %s`,
					host.DisplayName,
					service.DisplayName,
					err.Error(),
				),
			)
		} else {
			connectorStatus.Nagios.NewSuccess(
				fmt.Sprintf(
					`Successfully got Nagios service state for HOST="%s" SERVICE="%s"`,
					host.DisplayName,
					service.DisplayName,
				),
			)
		}
	}

	// check betterstack
	err = wh.betterStackApi.CheckIncidentsEndpoint()
	if err != nil {
		connectorStatus.BetterStack.NewFailure("Failed to check BetterStack incidents endpoint: " + err.Error())
	} else {
		connectorStatus.BetterStack.NewSuccess("BetterStack incidents endpoint returned status 200")
	}

	health := HEALTHY

	format_template, err := template.New("status").Parse(healthTextTemplate)
	if err != nil {
		health = UNHEALTHY
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to parse status template: " + err.Error()))
		return
	}

	if connectorStatus.Database.State == UNHEALTHY ||
		connectorStatus.Nagios.State == UNHEALTHY ||
		connectorStatus.BetterStack.State == UNHEALTHY {
		health = UNHEALTHY
	}

	if health == HEALTHY {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	err = format_template.Execute(w, connectorStatus)
	if err != nil {
		log.Println("ERROR Failed to write health status template: " + err.Error())
	}
}
