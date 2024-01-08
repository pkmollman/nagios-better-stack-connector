package betterstack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type BetterStackClient struct {
	apiKey  string
	baseUrl string
}

type BetterStackIncidentWebhookPayload struct {
	Data struct {
		Id         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
	} `json:"data"`
}

func NewBetterStackClient(apiKey, baseUrl string) *BetterStackClient {
	return &BetterStackClient{
		apiKey:  apiKey,
		baseUrl: baseUrl,
	}
}

func (b *BetterStackClient) NewRequest(httpMethod, endpoint string, data io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(httpMethod, b.baseUrl+endpoint, data)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+b.apiKey)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (b *BetterStackClient) CreateIncident(incidentName, incidentCause, alertId string) (string, error) {
	// create it
	var betterStackIncident struct {
		RequesterEmail     string `json:"requester_email"`
		IncidentName       string `json:"name"`
		Summary            string `json:"summary"`
		Description        string `json:"description"`
		CallOnCall         bool   `json:"call"`
		SMSOnCall          bool   `json:"sms"`
		EmailOnCall        bool   `json:"email"`
		PushOnCall         bool   `json:"push"`
		TeamWaitTime       *int   `json:"team_wait,omitempty"`
		EscalationPolicyId string `json:"policy_id"`
	}

	betterStackIncident.RequesterEmail = "mollman@uoregon.edu"
	betterStackIncident.IncidentName = incidentName
	betterStackIncident.Summary = incidentCause
	betterStackIncident.Description = incidentCause
	betterStackIncident.EscalationPolicyId = "94867"

	// marshal struct to json to reader
	jsonBody, err := json.Marshal(betterStackIncident)
	if err != nil {
		return "", err
	}
	jsonBodyReader := bytes.NewReader(jsonBody)

	req, err := b.NewRequest("POST", "/api/v2/incidents", jsonBodyReader)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// check response
	if res.StatusCode != 201 {
		return "", fmt.Errorf("response status code was %d", res.StatusCode)
	}

	var incidentResponse struct {
		Data struct {
			Id string `json:"id"`
		} `json:"data"`
	}

	err = json.NewDecoder(res.Body).Decode(&incidentResponse)
	if err != nil {
		return "", err
	}

	// return success
	return incidentResponse.Data.Id, nil
}
