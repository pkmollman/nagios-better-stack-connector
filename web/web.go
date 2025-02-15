package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/pkmollman/nagios-better-stack-connector/betterstack"
	"github.com/pkmollman/nagios-better-stack-connector/database"
	"github.com/pkmollman/nagios-better-stack-connector/database/sqlitedb"
	"github.com/pkmollman/nagios-better-stack-connector/nagios"
)

type webHandler struct {
	dbClient                       database.DatabaseClient
	betterClient                   *betterstack.BetterStackClient
	nagiosClient                   *nagios.NagiosClient
	BetterStackDefaultContactEmail string
	healthStatus                   nbscStatus
	healthStatusMutex              sync.Mutex
}

func NewWebHandler(dbClient database.DatabaseClient, betterStackClient *betterstack.BetterStackClient, nagiosClient *nagios.NagiosClient) *webHandler {
	handler := webHandler{
		dbClient:     dbClient,
		betterClient: betterStackClient,
		nagiosClient: nagiosClient,
	}

	handler.startHealthRoutine()

	return &handler
}

func StartServer() {
	// DB
	sqliteDbPath := getEnvVarOrPanic("SQLITE_DB_PATH")
	sqliteDbBackupDirPath := getEnvVarOrPanic("SQLITE_DB_BACKUP_DIR_PATH")
	sqliteDbBackupFrequencyMinutesString := getEnvVarOrPanic("SQLITE_DB_BACKUP_FREQUENCY_MINUTES")

	// convert string to int
	sqliteDbBackupFrequencyMinutes, err := strconv.Atoi(sqliteDbBackupFrequencyMinutesString)
	if err != nil {
		fmt.Println("unable to convert SQLITE_DB_BACKUP_FREQUENCY_MINUTES to int:", err)
		os.Exit(1)
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
		fmt.Println("unable to create database client:", err.Error())
		os.Exit(1)
	}

	err = dbClient.Init()
	if err != nil {
		fmt.Println("unable to initialize database client:", err.Error())
		os.Exit(1)
	}

	// start backup routine
	go func() {
		fmt.Println("Starting backup routine to backup every", sqliteDbBackupFrequencyMinutes, "minute(s)")
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

	// create HTTP router
	mux := http.NewServeMux()

	// Handle Incoming Nagios Notifications
	mux.HandleFunc("POST /api/nagios-event", webHandler.handleIncomingNagiosNotification)

	// Handle Incoming Better Stack Webhooks
	mux.HandleFunc("POST /api/better-stack-event", webHandler.handleIncomingBetterStackWebhook)

	// Handle Health Check
	mux.HandleFunc("GET /api/health", webHandler.handleHealthRequest)

	// Handle get event items
	mux.HandleFunc("GET /api/event-items", webHandler.handleGetEventItems)

	// HTTP server
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// run it in a goroutine so we can gracefully shutdown later
	go func() {
		fmt.Println("Listening on port 8080")
		lerr := httpServer.ListenAndServe()
		if lerr != nil && lerr != http.ErrServerClosed {
			fmt.Println("Error starting server:", lerr.Error())
			os.Exit(1)
		}
	}()

	// wait for signal to shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	fmt.Println("Server shutting down")

	// shutdown HTTP server
	httpShutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	herr := httpServer.Shutdown(httpShutdownContext)
	if herr != nil {
		fmt.Println("Error gracefully shutting down http server:", herr.Error())
	}

	// Wait for exclusive access to the database to backups and shutdown
	dbClient.Lock()
	dbClient.Backup()
	err = dbClient.Shutdown()
	if err != nil {
		fmt.Println(err)
	}

	if herr != nil || err != nil {
		os.Exit(1)
	}
}
