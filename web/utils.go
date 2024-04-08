package web

import (
	"fmt"
	"net/http"
	"os"
)




func getEnvVarOrPanic(key string) string {
	value := os.Getenv(key)
	if value == "" {
		fmt.Println("environment variable could not be found:", key)
		os.Exit(1)
	}

	return value
}

func logRequest(r *http.Request) {
	remoteAddr := r.RemoteAddr

	// if X-Forwarded-For header is set, use that instead
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		remoteAddr = forwardedFor
	}

	fmt.Println(fmt.Sprintf("INFO %s %s %s %s", remoteAddr, r.Method, r.URL, r.Proto))
}
