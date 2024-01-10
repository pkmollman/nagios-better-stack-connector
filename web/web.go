package web

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/pkmollman/nagios-better-stack-connector/betterstack"
	"github.com/pkmollman/nagios-better-stack-connector/database"
	"github.com/pkmollman/nagios-better-stack-connector/nagios"
)

func getEnvVarOrPanic(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s environment variable could not be found", key)
	}

	return value
}

func logRequest(r *http.Request) {
	slog.Info(fmt.Sprintf("%s %s %s", r.Method, r.URL, r.Proto))
}

func StartServer() {

	// COSMOS DB
	endpoint := getEnvVarOrPanic("AZURE_COSMOS_ENDPOINT")
	key := getEnvVarOrPanic("AZURE_COSMOS_KEY")
	databaseName := getEnvVarOrPanic("AZURE_COSMOS_DATABASE")
	containerName := getEnvVarOrPanic("AZURE_COSMOS_CONTAINER")

	// BetterStack
	betterStackApiKey := getEnvVarOrPanic("BETTER_STACK_API_KEY")

	// Nagios
	nagiosUser := getEnvVarOrPanic("NAGIOS_THRUK_API_USER")
	nagiosKey := getEnvVarOrPanic("NAGIOS_THRUK_API_KEY")
	nagiosBaseUrl := getEnvVarOrPanic("NAGIOS_THRUK_BASE_URL")
	nagiosSiteName := getEnvVarOrPanic("NAGIOS_THRUK_SITE_NAME")

	// connect to COSMOS DB
	cred, err := azcosmos.NewKeyCredential(key)
	if err != nil {
		log.Fatal("Failed to create a credential: ", err)
	}

	client, err := azcosmos.NewClientWithKey(endpoint, cred, nil)
	if err != nil {
		log.Fatal("Failed to create Azure Cosmos DB client: ", err)
	}

	// create betterstack client
	betterStackClient := betterstack.NewBetterStackClient(betterStackApiKey, "https://uptime.betterstack.com")

	// create nagios client
	nagiosClient := nagios.NewNagiosClient(nagiosUser, nagiosKey, nagiosBaseUrl, nagiosSiteName)

	/// Handle Incoming Nagios Notifications
	http.HandleFunc("/api/nagios-event", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		var event database.EventItem

		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		incidentName := "placeholder - incident name"

		if event.NagiosProblemServiceName != "" {
			incidentName = fmt.Sprintf("[%s] - [%s]", event.NagiosProblemHostname, event.NagiosProblemServiceName)
			event.NagiosProblemType = "SERVICE"
		} else {
			incidentName = fmt.Sprintf("[%s]", event.NagiosProblemHostname)
			event.NagiosProblemType = "HOST"
		}

		slog.Info("Incoming notification: " + incidentName + " problemId " + event.Id)

		switch event.NagiosProblemNotificationType {
		case "PROBLEM":
			// create it
			slog.Info("Creating incident: " + incidentName)
			betterStackIncidentId, err := betterStackClient.CreateIncident(incidentName, event.NagiosProblemContent, event.Id)
			if err != nil {
				slog.Error("Failed to create incident: " + incidentName)
				slog.Error(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			event.BetterStackIncidentId = betterStackIncidentId

			// create it
			err = database.CreateEventItem(client, databaseName, containerName, event.NagiosSiteName, event)
			if err != nil {
				slog.Error("Failed to create event item: " + incidentName)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			slog.Info("Created incident: " + incidentName)
		case "ACKNOWLEDGEMENT":
			items, _ := database.GetAllEventItems(client, databaseName, containerName, nagiosSiteName)

			for _, item := range items {
				if item.NagiosProblemId == event.NagiosProblemId {
					ackerr := betterStackClient.AcknowledgeIncident(item.BetterStackIncidentId)
					if ackerr != nil {
						slog.Error("Failed to acknowledge incident: " + incidentName)
						slog.Error(ackerr.Error())
						http.Error(w, ackerr.Error(), http.StatusInternalServerError)
						return
					} else {
						slog.Info("Acknowledged incident: " + incidentName + " " + item.BetterStackIncidentId)
					}
				}
			}
		case "RECOVERY":
			// update it
			// check if it is already recovered
			// problem ID will be 0, so you will need to associate the incident id by event hostname and service name
			slog.Info("Resolving incident logic goes here: " + incidentName)
			items, _ := database.GetAllEventItems(client, databaseName, containerName, nagiosSiteName)

			for _, item := range items {
				if item.NagiosProblemHostname == event.NagiosProblemHostname && item.NagiosProblemServiceName == event.NagiosProblemServiceName {
					ackerr := betterStackClient.ResolveIncident(item.BetterStackIncidentId)
					if ackerr != nil {
						slog.Error("Failed to resolve incident: " + incidentName)
						slog.Error(ackerr.Error())
						http.Error(w, ackerr.Error(), http.StatusInternalServerError)
						return
					} else {
						slog.Info("Resolved incident: " + incidentName + " " + item.BetterStackIncidentId)
					}
				}
			}
		default:
			// ignore it
			slog.Info("Ignoring incoming notification: " + incidentName + " STATUS " + event.NagiosProblemNotificationType)
		}

		// return success
		w.WriteHeader(http.StatusOK)
	})

	/// Handle Incoming Better Stack Webhooks
	http.HandleFunc("/api/better-stack-event", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		var event betterstack.BetterStackIncidentWebhookPayload

		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if event.Data.Attributes.Status == "acknowledged" || event.Data.Attributes.Status == "resolved" {
			var eventData database.EventItem

			items, _ := database.GetAllEventItems(client, databaseName, containerName, nagiosSiteName)

			for _, item := range items {
				if item.BetterStackIncidentId == event.Data.Id {
					eventData = item
				}
			}

			if eventData.Id == "" {
				slog.Error("Could not find event for betterstack incident id: " + event.Data.Id)
				http.Error(w, "Could not find event", http.StatusBadRequest)
				return
			} else {
				// check if it is already acknowledged or recovered
				serviceState, err := nagiosClient.GetServiceState(eventData.NagiosProblemHostname, eventData.NagiosProblemServiceName)
				if err != nil {
					slog.Error("Failed to get service ack state: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if serviceState.Acknowledged == 0 && serviceState.State != 0 {
					err = nagiosClient.AckService(eventData.NagiosProblemHostname, eventData.NagiosProblemServiceName, "Acknowledged by BetterStack")
					if err != nil {
						slog.Error("Failed to acknowledge service: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					} else {
						slog.Info("Acknowledged service: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
					}
				} else {
					slog.Info("Service already acknowledged, or recovered: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
				}
			}
		}

		// return success
		w.WriteHeader(http.StatusOK)
	})

	// serve
	goFuncPort := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if goFuncPort != "" {
		fmt.Println("Running in Azure Functions")
		fmt.Println("Listening on port " + goFuncPort)
		log.Fatal(http.ListenAndServe(":"+goFuncPort, nil))
	} else {
		fmt.Println("Listening on port 8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
