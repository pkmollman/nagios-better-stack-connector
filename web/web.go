package web

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	log.Println(fmt.Sprintf("INFO %s %s %s", r.Method, r.URL, r.Proto))
}

func StartServer() {

	// DB
	sqliteDbPath := getEnvVarOrPanic("SQLITE_DB_PATH")

	// BetterStack
	betterStackApiKey := getEnvVarOrPanic("BETTER_STACK_API_KEY")
	betterDefaultContactEmail := getEnvVarOrPanic("BETTER_STACK_DEFAULT_CONTACT_EMAIL")

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
	betterStackClient := betterstack.NewBetterStackClient(betterStackApiKey, "https://uptime.betterstack.com")

	// create nagios client
	nagiosClient := nagios.NewNagiosClient(nagiosUser, nagiosKey, nagiosBaseUrl, nagiosSiteName)

	http.HandleFunc("GET /api/nagios-event", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
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

		incidentName := "placeholder - incident name"

		// identify event as either host or service problem
		if event.NagiosProblemServiceName != "" {
			incidentName = fmt.Sprintf("[%s] - [%s]", event.NagiosProblemHostname, event.NagiosProblemServiceName)
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
			events, err := dbClient.GetAllEventItems()
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
			betterStackIncidentId, err := betterStackClient.CreateIncident(event.BetterStackPolicyId, betterDefaultContactEmail, incidentName, event.NagiosProblemContent, event.Id)
			if err != nil {
				log.Println("ERROR Failed to create incident: " + incidentName + " " + err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			event.BetterStackIncidentId = betterStackIncidentId

			err = dbClient.CreateEventItem(event)
			if err != nil {
				log.Println("ERROR Failed to create event item: " + incidentName + " " + err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			log.Println("INFO Created incident: " + incidentName)
		case "ACKNOWLEDGEMENT":
			items, _ := dbClient.GetAllEventItems()

			for _, item := range items {
				if item.NagiosProblemId == event.NagiosProblemId &&
					item.NagiosSiteName == event.NagiosSiteName &&
					item.NagiosProblemHostname == event.NagiosProblemHostname &&
					item.NagiosProblemServiceName == event.NagiosProblemServiceName &&
					item.NagiosProblemType == event.NagiosProblemType &&
					item.BetterStackPolicyId == event.BetterStackPolicyId {
					ackerr := betterStackClient.AcknowledgeIncident(event.InteractingUserEmail, betterDefaultContactEmail, item.BetterStackIncidentId)
					if ackerr != nil {
						log.Println("ERROR Failed to acknowledge incident: " + incidentName + " " + err.Error())
						http.Error(w, ackerr.Error(), http.StatusInternalServerError)
						return
					} else {
						log.Println("INFO Acknowledged incident: " + incidentName + " " + item.BetterStackIncidentId)
					}
				}
			}
		case "RECOVERY":
			items, err := dbClient.GetAllEventItems()
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
					ackerr := betterStackClient.ResolveIncident(event.InteractingUserEmail, betterDefaultContactEmail, item.BetterStackIncidentId)
					if ackerr != nil {
						log.Println("ERROR Failed to resolve incident: " + incidentName + " " + err.Error())
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
	})

	/// Handle Incoming Better Stack Webhooks
	http.HandleFunc("POST /api/better-stack-event", func(w http.ResponseWriter, r *http.Request) {
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
			var eventData models.EventItem

			items, err := dbClient.GetAllEventItems()
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

			if eventData.Id == "" {
				log.Println("ERROR Could not find event for betterstack incident id: " + event.Data.Id)
				http.Error(w, "Could not find event", http.StatusBadRequest)
				return
			} else {
				switch eventData.NagiosProblemType {
				case "HOST":
					// check if it is already acknowledged or recovered
					hostState, err := nagiosClient.GetHostState(eventData.NagiosProblemHostname)
					if err != nil {
						log.Println("ERROR Failed to get host ack state: " + eventData.NagiosProblemHostname)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					if hostState.Acknowledged == 0 && hostState.State != 0 {
						err = nagiosClient.AckHost(eventData.NagiosProblemHostname, "Acknowledged by BetterStack")
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
					serviceState, err := nagiosClient.GetServiceState(eventData.NagiosProblemHostname, eventData.NagiosProblemServiceName)
					if err != nil {
						log.Println("ERROR Failed to get service ack state: " + eventData.NagiosProblemHostname + " " + eventData.NagiosProblemServiceName)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					if serviceState.Acknowledged == 0 && serviceState.State != 0 {
						err = nagiosClient.AckService(eventData.NagiosProblemHostname, eventData.NagiosProblemServiceName, "Acknowledged by BetterStack")
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
	})

	// Log to syslog
	// logWriter, err := syslog.New(syslog.LOG_SYSLOG, "Nagios BetterStack Connector")
	// if err != nil {
	// 	log.Fatalln("Unable to set logfile:", err.Error())
	// }
	// // set the log output
	// log.SetOutput(logWriter)

	go func() {
		log.Println("Listening on port 8080")
		log.Fatal(http.ListenAndServe(":8080", nil))

	}()

	// wait for signal to shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	log.Println("Server shutting down")
	err = dbClient.Shutdown()
	if err != nil {
		log.Fatal(err)
	}
}
