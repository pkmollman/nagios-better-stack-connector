package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "modernc.org/sqlite"

	"github.com/pkmollman/nagios-better-stack-connector/betterstack"
	"github.com/pkmollman/nagios-better-stack-connector/database"
	"github.com/pkmollman/nagios-better-stack-connector/database/sqlitedb"
	"github.com/pkmollman/nagios-better-stack-connector/models"
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

	// DB
	sqliteDbPath := getEnvVarOrPanic("SQLITE_DB_PATH")

	// BetterStack
	betterStackApiKey := getEnvVarOrPanic("BETTER_STACK_API_KEY")
	betterContactEmail := getEnvVarOrPanic("BETTER_STACK_CONTACT_EMAIL")

	// Nagios
	nagiosUser := getEnvVarOrPanic("NAGIOS_THRUK_API_USER")
	nagiosKey := getEnvVarOrPanic("NAGIOS_THRUK_API_KEY")
	nagiosBaseUrl := getEnvVarOrPanic("NAGIOS_THRUK_BASE_URL")
	nagiosSiteName := getEnvVarOrPanic("NAGIOS_THRUK_SITE_NAME")

	// // connect to COSMOS DB
	// cred, err := azcosmos.NewKeyCredential(key)
	// if err != nil {
	// 	log.Fatal("Failed to create a credential: ", err)
	// }

	// client, err := azcosmos.NewClientWithKey(endpoint, cred, nil)
	// if err != nil {
	// 	log.Fatal("Failed to create Azure Cosmos DB client: ", err)
	// }
	//

	// sqlite
	db, err := sql.Open("sqlite", sqliteDbPath)
	if err != nil {
		log.Fatal(err)
	}

	var dbClient database.DatabaseClient = &sqlitedb.SqlliteClient{}
	dbClient.Init(db)

	// create betterstack client
	_ = betterstack.NewBetterStackClient(betterStackApiKey, "https://uptime.betterstack.com")

	// create nagios client
	nagiosClient := nagios.NewNagiosClient(nagiosUser, nagiosKey, nagiosBaseUrl, nagiosSiteName)

	http.HandleFunc("GET /api/nagios-event", func(w http.ResponseWriter, r *http.Request) {
		dbClient.Lock()
		defer dbClient.Unlock()
		events, err := dbClient.GetAllEventItems()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(events)

	})

	/// Handle Incoming Nagios Notifications
	http.HandleFunc("POST /api/nagios-event", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		dbClient.Lock()
		defer dbClient.Unlock()
		var event models.EventItem

		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		incidentName := "placeholder - incident name"

		// identify event as either host or service problem
		if event.NagiosProblemServiceName != "" {
			incidentName = fmt.Sprintf("[%s] - [%s]", event.NagiosProblemHostname, event.NagiosProblemServiceName)
			event.NagiosProblemType = "SERVICE"
		} else {
			incidentName = fmt.Sprintf("[%s]", event.NagiosProblemHostname)
			event.NagiosProblemType = "HOST"
		}

		slog.Info("Incoming notification: " + incidentName + " problemId " + event.Id)

		// handle creating indicents for new problems, and acking/resolving existing problems
		// TODO - handle incoming notifications for existing incidents
		switch event.NagiosProblemNotificationType {
		case "PROBLEM":
			// check if incident already exists
			events, err := dbClient.GetAllEventItems()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			for _, e := range events {
				if e.NagiosProblemId == event.NagiosProblemId &&
					e.NagiosProblemType == event.NagiosProblemType &&
					e.NagiosSiteName == event.NagiosSiteName &&
					e.BetterStackPolicyId == event.BetterStackPolicyId {
					slog.Info("Ignoring superfluous nagios notification for incident: \"" + incidentName + "\"")
					w.WriteHeader(http.StatusOK)
					return
				}
			}

			slog.Info("Creating incident: " + incidentName)
			// betterStackIncidentId, err := betterStackClient.CreateIncident(incidentName, event.NagiosProblemContent, event.Id)
			// if err != nil {
			// 	slog.Error("Failed to create incident: " + incidentName)
			// 	slog.Error(err.Error())
			// 	http.Error(w, err.Error(), http.StatusInternalServerError)
			// 	return
			// }

			betterStackIncidentId := "placeholder - betterstack incident id"
			event.BetterStackIncidentId = betterStackIncidentId

			err = dbClient.CreateEventItem(event)
			if err != nil {
				slog.Error("Failed to create event item: " + incidentName)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			slog.Info("Created incident: " + incidentName)
		case "ACKNOWLEDGEMENT":
			items, _ := dbClient.GetAllEventItems()

			for _, item := range items {
				if item.NagiosProblemId == event.NagiosProblemId &&
					item.NagiosSiteName == event.NagiosSiteName {
					// ackerr := betterStackClient.AcknowledgeIncident(item.BetterStackIncidentId)
					var ackerr error = nil
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
			items, err := dbClient.GetAllEventItems()
			if err != nil {
				slog.Error("Failed to get all event items")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			for _, item := range items {
				if item.NagiosProblemType == event.NagiosProblemType &&
					item.NagiosSiteName == event.NagiosSiteName &&
					item.NagiosProblemHostname == event.NagiosProblemHostname &&
					item.NagiosProblemServiceName == event.NagiosProblemServiceName {
					// ackerr := betterStackClient.ResolveIncident(item.BetterStackIncidentId)
					var ackerr error = nil
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

		// ack nagios services/host problems based off incident ID, only act on acknowledged and resolved events
		if event.Data.Attributes.Status == "acknowledged" || event.Data.Attributes.Status == "resolved" {
			var eventData models.EventItem

			items, err := dbClient.GetAllEventItems()
			if err != nil {
				slog.Error("Failed to get all event items")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

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
				switch eventData.NagiosProblemType {
				case "HOST":
					// host logic
					// check if it is already acknowledged or recovered
					hostState, err := nagiosClient.GetHostState(eventData.NagiosProblemHostname)
					if err != nil {
						slog.Error("Failed to get host ack state: " + eventData.NagiosProblemHostname)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					if hostState.Acknowledged == 0 && hostState.State != 0 {
						err = nagiosClient.AckHost(eventData.NagiosProblemHostname, "Acknowledged by BetterStack")
						if err != nil {
							slog.Error("Failed to acknowledge host: " + eventData.NagiosProblemHostname)
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						} else {
							slog.Info("Acknowledged host: " + eventData.NagiosProblemHostname)
						}
					} else {
						slog.Info("Host already acknowledged, or recovered: " + eventData.NagiosProblemHostname)
					}

				case "SERVICE":
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
		}

		// return success
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		fmt.Println("Listening on port 8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// wait for signal to shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	fmt.Println("Server shutting down")
	err = dbClient.Shutdown()
	if err != nil {
		log.Fatal(err)
	}
}
