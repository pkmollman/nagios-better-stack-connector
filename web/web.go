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
	nagios.NewNagiosClient(nagiosUser, nagiosKey, nagiosBaseUrl, nagiosSiteName)

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

		betterStackIncidentId, err := betterStackClient.CreateIncident(incidentName, event.NagiosProblemContent, event.Id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		event.BetterStackIncidentId = betterStackIncidentId

		// create it
		err = database.CreateEventItem(client, databaseName, containerName, event.NagiosSiteName, event)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
			eventData := database.EventItem{}

			items, _ := database.GetAllEventItems(client, databaseName, containerName, nagiosSiteName)

			for _, item := range items {
				if item.BetterStackIncidentId == event.Data.Id {
					eventData = item
				}
			}
			fmt.Println("would ack nagios:")
			fmt.Println(eventData)
			//nagiosClient.AckService(eventData.NagiosProblemHostname, eventData.NagiosProblemServiceName, "Acknowledged by BetterStack")
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
