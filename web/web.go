package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/template"

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
	log.Println(fmt.Sprintf("INFO %s %s %s %s", r.RemoteAddr, r.Method, r.URL, r.Proto))
}

type WebHandler struct {
	dbClient                       database.DatabaseClient
	betterStackApi                 *betterstack.BetterStackClient
	nagiosClient                   *nagios.NagiosClient
	BetterStackDefaultContactEmail string
}

func NewWebHandler(dbClient database.DatabaseClient, betterStackApi *betterstack.BetterStackClient, nagiosClient *nagios.NagiosClient) *WebHandler {
	return &WebHandler{
		dbClient:       dbClient,
		betterStackApi: betterStackApi,
		nagiosClient:   nagiosClient,
	}
}

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

		err = wh.dbClient.CreateEventItem(event)
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
					log.Println("ERROR Failed to acknowledge incident: " + incidentName + " " + err.Error())
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
}

func (wh *WebHandler) handleIncomingBetterStackWebhook(w http.ResponseWriter, r *http.Request) {
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

		if eventData.Id == "" {
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

func (wh *WebHandler) handleHealthRequest(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.Header().Set("Content-Type", "text/plain")
	const (
		healthy   = "healthy"
		unhealthy = "unhealthy"
	)

	type nbsc_service_status struct {
		State  string   `json:"state"`
		Errors []string `json:"errors"`
	}

	nbsc_status := struct {
		Database    nbsc_service_status `json:"database"`
		Nagios      nbsc_service_status `json:"nagios"`
		BetterStack nbsc_service_status `json:"betterstack"`
	}{
		Database: nbsc_service_status{
			State: healthy,
		},
		Nagios: nbsc_service_status{
			State: healthy,
		},
		BetterStack: nbsc_service_status{
			State: healthy,
		},
	}

	status_text_template := `
Database: {{.Database.State}}
{{range .Database.Errors}}  - {{.}}
{{end}}
Nagios: {{.Nagios.State}}
{{range .Nagios.Errors}}  - {{.}}
{{end}}
BetterStack: {{.BetterStack.State}}
{{range .BetterStack.Errors}}  - {{.}}
{{end}}
`

	// check database
	wh.dbClient.Lock()
	_, err := wh.dbClient.GetAllEventItems()
	wh.dbClient.Unlock()
	if err != nil {
		nbsc_status.Database.State = unhealthy
		nbsc_status.Database.Errors = append(
			nbsc_status.Database.Errors,
			"Failed to get event items from database: "+err.Error(),
		)
	}

	// check nagios
	hosts, err := wh.nagiosClient.GetHosts()
	if err != nil {
		nbsc_status.Nagios.State = unhealthy
		nbsc_status.Nagios.Errors = append(
			nbsc_status.Nagios.Errors,
			"Failed to get hosts from Nagios: "+err.Error(),
		)
	}

	// pick a random host
	if err != nil && len(hosts) > 0 {
		host := hosts[rand.Intn(len(hosts)-1)]

		for len(host.Services) == 0 {
			host = hosts[rand.Intn(len(hosts)-1)]
		}

		// check service
		service, err := wh.nagiosClient.GetServiceState(host.DisplayName, host.Services[0])
		if err != nil {
			nbsc_status.Nagios.State = unhealthy
			nbsc_status.Nagios.Errors = append(
				nbsc_status.Nagios.Errors,
				fmt.Sprintf(`Failed to get Nagios service state for "%s" > "%s": %s`, host.DisplayName, service.DisplayName, err.Error()),
			)
		}
	}

	// check betterstack
	err = wh.betterStackApi.CheckIncidentsEndpoint()
	if err != nil {
		nbsc_status.BetterStack.State = unhealthy
		nbsc_status.BetterStack.Errors = append(
			nbsc_status.BetterStack.Errors,
			"Failed to check BetterStack incidents endpoint: "+err.Error(),
		)
	}

	health := healthy

	format_template, err := template.New("status").Parse(status_text_template)
	if err != nil {
		health = unhealthy
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to parse status template"))
		return
	}

	if nbsc_status.Database.State == unhealthy ||
		nbsc_status.Nagios.State == unhealthy ||
		nbsc_status.BetterStack.State == unhealthy {
		health = unhealthy
	}

	if health == healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	err = format_template.Execute(w, nbsc_status)
	if err != nil {
		log.Println("ERROR Failed to write health status template: " + err.Error())
	}
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

	// create database client
	var dbClient database.DatabaseClient

	// should be able to swap this out for anything that implements the database.DatabaseClient interface
	dbClient, err := sqlitedb.NewSQLiteClient(sqliteDbPath)
	if err != nil {
		log.Fatalf("unable to create database client: %s", err.Error())
	}

	err = dbClient.Init()
	if err != nil {
		log.Fatalf("unable to initialize database client: %s", err.Error())
	}

	// create betterstack client
	betterStackClient := betterstack.NewBetterStackClient(betterStackApiKey, "https://uptime.betterstack.com")

	// create nagios client
	nagiosClient := nagios.NewNagiosClient(nagiosUser, nagiosKey, nagiosBaseUrl, nagiosSiteName)

	webHandler := NewWebHandler(dbClient, betterStackClient, nagiosClient)
	webHandler.BetterStackDefaultContactEmail = betterDefaultContactEmail

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/nagios-event", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("POST /api/nagios-event", webHandler.handleIncomingNagiosNotification)
	/// Handle Incoming Better Stack Webhooks
	mux.HandleFunc("POST /api/better-stack-event", webHandler.handleIncomingBetterStackWebhook)
	/// Handle Health Check
	mux.HandleFunc("GET /api/health", webHandler.handleHealthRequest)

	go func() {
		log.Println("Listening on port 8080")
		log.Fatal(http.ListenAndServe(":8080", mux))
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
