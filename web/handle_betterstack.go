package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pkmollman/nagios-better-stack-connector/betterstack"
	"github.com/pkmollman/nagios-better-stack-connector/models"
)

func (wh *webHandler) handleIncomingBetterStackWebhook(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	var event betterstack.BetterStackIncidentWebhookPayload

	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		log.Println("ERROR Failed to decode better stack playload: " + err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// ack nagios services/host problems based off incident ID, only act on acknowledged and resolved events
	if event.Data.Attributes.Status == "acknowledged" || event.Data.Attributes.Status == "resolved" {
		fmt.Println("Before trying to lock DB in BS handler")
		wh.dbClient.Lock()
		fmt.Println("Got lock DB in BS handler")
		defer wh.dbClient.Unlock()

		var eventData models.EventItem

		items, err := wh.dbClient.GetAllEventItems()
		if err != nil {
			log.Println("ERROR Failed to get all event items: " + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, item := range items {
			if item.BetterStackIncidentId == event.Data.Id {
				eventData = item
			}
		}

		if eventData.BetterStackIncidentId == "" {
			log.Println("ERROR Could not find event for betterstack incident id: " + event.Data.Id)
			http.Error(w, "Could not find event", http.StatusBadRequest)
			return
		} else {
			switch eventData.NagiosProblemType {
			case "HOST":
				// check if it is already acknowledged or recovered
				hostState, err := wh.nagiosClient.GetHostState(eventData.NagiosProblemHostname)
				if err != nil {
					log.Println("ERROR Failed to get host ack state: " + eventData.NagiosProblemHostname)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if hostState.Acknowledged == 0 && hostState.State != 0 {
					err = wh.nagiosClient.AckHost(eventData.NagiosProblemHostname, "Acknowledged by BetterStack")
					if err != nil {
						log.Println("ERROR Failed to acknowledge host: " + eventData.NagiosProblemHostname)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					} else {
						log.Println("INFO Acknowledged host: " + eventData.NagiosProblemHostname)
					}
				} else {
					log.Println("INFO Host already acknowledged, or recovered: " + eventData.NagiosProblemHostname)
				}

			case "SERVICE":
				// check if it is already acknowledged or recovered
				serviceState, err := wh.nagiosClient.GetServiceState(eventData.NagiosProblemHostname, eventData.NagiosProblemServiceName)
				if err != nil {
					log.Println("ERROR Failed to get service ack state: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if serviceState.Acknowledged == 0 && serviceState.State != 0 {
					err = wh.nagiosClient.AckService(eventData.NagiosProblemHostname, eventData.NagiosProblemServiceName, "Acknowledged by BetterStack")
					if err != nil {
						log.Println("ERROR Failed to acknowledge service: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					} else {
						log.Println("INFO Acknowledged service: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
					}
				} else {
					log.Println("INFO Service already acknowledged, or recovered: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
				}
			}
		}
	}

	// return success
	w.WriteHeader(http.StatusOK)
}
