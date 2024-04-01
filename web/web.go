package web

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pkmollman/nagios-better-stack-connector/betterstack"
	"github.com/pkmollman/nagios-better-stack-connector/database"
	"github.com/pkmollman/nagios-better-stack-connector/database/sqlitedb"
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

func StartServer() {
	// DB
	sqliteDbPath := getEnvVarOrPanic("SQLITE_DB_PATH")
	sqliteDbBackupDirPath := getEnvVarOrPanic("SQLITE_DB_BACKUP_DIR_PATH")
	sqliteDbBackupFrequencyMinutesString := getEnvVarOrPanic("SQLITE_DB_BACKUP_FREQUENCY_MINUTES")

	// convert string to int
	sqliteDbBackupFrequencyMinutes, err := strconv.Atoi(sqliteDbBackupFrequencyMinutesString)
	if err != nil {
		log.Fatalf("unable to convert SQLITE_DB_BACKUP_FREQUENCY_MINUTES to int: %s", err)
	}

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
	dbClient, err = sqlitedb.NewSQLiteClient(sqliteDbPath, sqliteDbBackupDirPath)
	if err != nil {
		log.Fatalf("unable to create database client: %s", err.Error())
	}

	err = dbClient.Init()
	if err != nil {
		log.Fatalf("unable to initialize database client: %s", err.Error())
	}

	// start backup routine
	go func() {
		log.Println("Starting backup routine to backup every", sqliteDbBackupFrequencyMinutes, "minute(s)")
		for {
			time.Sleep(time.Minute * time.Duration(sqliteDbBackupFrequencyMinutes))
			func() {
				dbClient.Lock()
				defer dbClient.Unlock()
				dbClient.Backup()
			}()
		}
	}()

	// create betterstack client
	betterStackClient := betterstack.NewBetterStackClient(betterStackApiKey, "https://uptime.betterstack.com")

	// create nagios client
	nagiosClient := nagios.NewNagiosClient(nagiosUser, nagiosKey, nagiosBaseUrl, nagiosSiteName)

	webHandler := NewWebHandler(dbClient, betterStackClient, nagiosClient)
	webHandler.BetterStackDefaultContactEmail = betterDefaultContactEmail

	mux := http.NewServeMux()

	// Handle Incoming Nagios Notifications
	mux.HandleFunc("POST /api/nagios-event", webHandler.handleIncomingNagiosNotification)

	// Handle Incoming Better Stack Webhooks
	mux.HandleFunc("POST /api/better-stack-event", webHandler.handleIncomingBetterStackWebhook)

	// Handle Health Check
	mux.HandleFunc("GET /api/health", webHandler.handleHealthRequest)

	// Handle get event items
	mux.HandleFunc("GET /api/event-items", webHandler.handleGetEventItems)

	go func() {
		log.Println("Listening on port 8080")
		log.Fatal(http.ListenAndServe(":8080", mux))
	}()

	// wait for signal to shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	log.Println("Server shutting down")
	dbClient.Lock()
	dbClient.Backup()
	err = dbClient.Shutdown()
	if err != nil {
		log.Fatal(err)
	}
}
