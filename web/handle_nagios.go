package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/pkmollman/nagios-better-stack-connector/models"
)

func (wh *WebHandler) handleIncomingNagiosNotification(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	wh.dbClient.Lock()
	wh.dbClient.Unlock()
	var event models.EventItem

	// body to string
	bodyBytes, err := io.ReadAll(r.Body)

	bodyString := string(bodyBytes)

	// reader from body bytes
	bodyReader := bytes.NewReader(bodyBytes)

	err = json.NewDecoder(bodyReader).Decode(&event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if event.NagiosSiteName == "" ||
		event.NagiosProblemNotificationType == "" ||
		event.NagiosProblemHostname == "" ||
		event.BetterStackPolicyId == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		log.Println("INFO Missing required fields, ignoring: " + bodyString)
		return
	}

	incidentName := "placeholder - incident name - something went wrong"

	// identify event as either host or service problem
	if event.NagiosProblemServiceName != "" {

		serviceName := event.NagiosProblemServiceName

		if strings.TrimSpace(event.NagiosProblemServiceDisplayName) != "" {
			serviceName = event.NagiosProblemServiceDisplayName
		}

		incidentName = fmt.Sprintf("[%s] - [%s]", event.NagiosProblemHostname, serviceName)
		event.NagiosProblemType = "SERVICE"
	} else {
		incidentName = fmt.Sprintf("[%s]", event.NagiosProblemHostname)
		event.NagiosProblemType = "HOST"
	}

	log.Println("INFO Incoming notification: " + incidentName + " problemId " + event.Id)

	// handle creating indicents for new problems, and acking/resolving existing problems
	switch event.NagiosProblemNotificationType {
	case "PROBLEM":
		if event.NagiosProblemId == "" {
			http.Error(w, "Missing required field \"nagiosProblemId\"", http.StatusBadRequest)
			return
		}
		// check if incident already exists
		events, err := wh.dbClient.GetAllEventItems()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, item := range events {
			if item.NagiosProblemId == event.NagiosProblemId &&
				item.NagiosProblemType == event.NagiosProblemType &&
				item.NagiosSiteName == event.NagiosSiteName &&
				item.BetterStackPolicyId == event.BetterStackPolicyId {
				log.Println("INFO Ignoring superfluous nagios notification for incident: \"" + incidentName + "\"")
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		log.Println("INFO Creating incident: " + incidentName)
		betterStackIncidentId, err := wh.betterStackApi.CreateIncident(event.BetterStackPolicyId, wh.BetterStackDefaultContactEmail, incidentName, event.NagiosProblemContent, event.Id)
		if err != nil {
			log.Println("ERROR Failed to create incident: " + incidentName + " " + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		event.BetterStackIncidentId = betterStackIncidentId

		_, err = wh.dbClient.CreateEventItem(event)
		if err != nil {
			log.Println("ERROR Failed to create event item: " + incidentName + " " + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Println("INFO Created incident: " + incidentName)
	case "ACKNOWLEDGEMENT":
		items, _ := wh.dbClient.GetAllEventItems()

		for _, item := range items {
			if item.NagiosProblemId == event.NagiosProblemId &&
				item.NagiosSiteName == event.NagiosSiteName &&
				item.NagiosProblemHostname == event.NagiosProblemHostname &&
				item.NagiosProblemServiceName == event.NagiosProblemServiceName &&
				item.NagiosProblemType == event.NagiosProblemType &&
				item.BetterStackPolicyId == event.BetterStackPolicyId {
				ackerr := wh.betterStackApi.AcknowledgeIncident(event.InteractingUserEmail, wh.BetterStackDefaultContactEmail, item.BetterStackIncidentId)
				if ackerr != nil {
					log.Println("WARN Failed to acknowledge incident: " + incidentName + " " + ackerr.Error())
					http.Error(w, ackerr.Error(), http.StatusInternalServerError)
					return
				} else {
					log.Println("INFO Acknowledged incident: " + incidentName + " " + item.BetterStackIncidentId)
				}
			}
		}
	case "RECOVERY":
		items, err := wh.dbClient.GetAllEventItems()
		if err != nil {
			log.Println("ERROR Failed to get all event items: " + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, item := range items {
			if item.NagiosProblemType == event.NagiosProblemType &&
				item.NagiosSiteName == event.NagiosSiteName &&
				item.NagiosProblemHostname == event.NagiosProblemHostname &&
				item.NagiosProblemServiceName == event.NagiosProblemServiceName &&
				item.BetterStackPolicyId == event.BetterStackPolicyId {
				ackerr := wh.betterStackApi.ResolveIncident(event.InteractingUserEmail, wh.BetterStackDefaultContactEmail, item.BetterStackIncidentId)
				if ackerr != nil {
					log.Println("WARN Failed to resolve incident: " + incidentName + " " + ackerr.Error())
					http.Error(w, ackerr.Error(), http.StatusInternalServerError)
					return
				} else {
					log.Println("INFO Resolved incident: " + incidentName + " " + item.BetterStackIncidentId)
				}
			}
		}
	default:
		// ignore it
		log.Println("INFO Ignoring incoming notification: " + incidentName + " STATUS " + event.NagiosProblemNotificationType)
	}

	// return success
	w.WriteHeader(http.StatusOK)
}
