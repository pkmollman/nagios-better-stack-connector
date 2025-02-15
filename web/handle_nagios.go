package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkmollman/nagios-better-stack-connector/models"
)

func (wh *webHandler) handleIncomingNagiosNotification(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	wh.dbClient.Lock()
	defer wh.dbClient.Unlock()
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
		fmt.Println("INFO Missing required fields, ignoring: " + bodyString)
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

	fmt.Println("INFO Incoming notification: " + incidentName + " nagiosProblemId " + event.NagiosProblemId)

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
				fmt.Println("INFO Ignoring superfluous nagios notification for incident: \"" + incidentName + "\"")
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		fmt.Println("INFO Creating incident: " + incidentName)
		betterStackIncidentId, err := wh.betterClient.CreateIncident(event.BetterStackPolicyId, wh.BetterStackDefaultContactEmail, incidentName, event.NagiosProblemContent)
		if err != nil {
			fmt.Println("ERROR Failed to create incident: " + incidentName + " " + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		event.BetterStackIncidentId = betterStackIncidentId

		_, err = wh.dbClient.CreateEventItem(event)
		if err != nil {
			fmt.Println("ERROR Failed to create event item: " + incidentName + " " + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Println("INFO Created incident: " + incidentName)
	case "ACKNOWLEDGEMENT":
		items, _ := wh.dbClient.GetAllEventItems()

		for _, item := range items {
			if item.NagiosProblemId == event.NagiosProblemId &&
				item.NagiosSiteName == event.NagiosSiteName &&
				item.NagiosProblemHostname == event.NagiosProblemHostname &&
				item.NagiosProblemServiceName == event.NagiosProblemServiceName &&
				item.NagiosProblemType == event.NagiosProblemType &&
				item.BetterStackPolicyId == event.BetterStackPolicyId {
				ackerr := wh.betterClient.AcknowledgeIncident(event.InteractingUserEmail, wh.BetterStackDefaultContactEmail, item.BetterStackIncidentId)
				if ackerr != nil {
					fmt.Println("WARN Failed to acknowledge incident: " + incidentName + " BetterStack incident ID " + item.BetterStackIncidentId + " " + ackerr.Error())
				} else {
					fmt.Println("INFO Acknowledged incident: " + incidentName + " BetterStack incident ID " + item.BetterStackIncidentId)
				}
			}
		}
	case "RECOVERY":
		items, err := wh.dbClient.GetAllEventItems()
		if err != nil {
			fmt.Println("ERROR Failed to get all event items: " + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, item := range items {
			if item.NagiosProblemType == event.NagiosProblemType &&
				item.NagiosSiteName == event.NagiosSiteName &&
				item.NagiosProblemHostname == event.NagiosProblemHostname &&
				item.NagiosProblemServiceName == event.NagiosProblemServiceName &&
				item.BetterStackPolicyId == event.BetterStackPolicyId {
				ackerr := wh.betterClient.ResolveIncident(event.InteractingUserEmail, wh.BetterStackDefaultContactEmail, item.BetterStackIncidentId)
				if ackerr != nil {
					fmt.Println("WARN Failed to resolve incident: " + incidentName + " BetterStack incident ID " + item.BetterStackIncidentId + " " + ackerr.Error())
				} else {
					fmt.Println("INFO Resolved incident: " + incidentName + " BetterStack incident ID " + item.BetterStackIncidentId)
				}
				_, delerr := wh.dbClient.DeleteEventItem(item.Id)
				if delerr != nil {
					fmt.Println(fmt.Sprintf("ERROR Failed to delete event item: %s ID %d %s", incidentName, item.Id, delerr.Error()))
				} else {
					fmt.Println(fmt.Sprintf("INFO Deleted event item: %s ID %d", incidentName, item.Id))
				}
			}
		}
	default:
		// ignore it
		fmt.Println("INFO Ignoring incoming notification: " + incidentName + " STATUS " + event.NagiosProblemNotificationType)
	}

	// return success
	w.WriteHeader(http.StatusOK)
}
