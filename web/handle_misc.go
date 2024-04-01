package web

import (
	"encoding/json"
	"net/http"
)

func (wh *WebHandler) handleGetEventItems(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	wh.dbClient.Lock()
	defer wh.dbClient.Unlock()
	events, err := wh.dbClient.GetAllEventItems()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(events)

}
