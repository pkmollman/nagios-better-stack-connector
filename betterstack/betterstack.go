package betterstack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const MAX_RETRIES int = 10

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

func (b *BetterStackClient) Do(req *http.Request) (*http.Response, error) {
	request_success := false
	wait_time_seconds := 0
	retries := 0
	var res *http.Response

	for request_success == false {
		if retries > MAX_RETRIES {
			return nil, fmt.Errorf("Hit max retries...")
		}
		if wait_time_seconds > 0 {
			retries++
			fmt.Println("Got 429 waiting for", wait_time_seconds, "seconds.")
			time.Sleep(time.Duration(wait_time_seconds) * time.Second)
		}
		var err error
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		if res.StatusCode == 429 {
			retry_after := res.Header.Get("Retry-After")
			if retry_after != "" {
				wait_time, perr := strconv.Atoi(retry_after)
				if perr != nil {
					wait_time_seconds = 5
				} else {
					wait_time_seconds = wait_time
				}
			}
		} else {
			request_success = true
		}
	}
	return res, nil
}

func (b *BetterStackClient) CreateIncident(escalation_policy, contact_email, incidentName, incidentCause string) (string, error) {
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

	betterStackIncident.RequesterEmail = contact_email
	betterStackIncident.IncidentName = incidentName
	betterStackIncident.Summary = incidentCause
	betterStackIncident.Description = incidentCause
	betterStackIncident.EscalationPolicyId = escalation_policy

	jsonBody, err := json.Marshal(betterStackIncident)
	if err != nil {
		return "", err
	}
	jsonBodyReader := bytes.NewReader(jsonBody)

	req, err := b.NewRequest("POST", "/api/v2/incidents", jsonBodyReader)

	res, err := b.Do(req)
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

func (b *BetterStackClient) AcknowledgeIncident(contact_email, default_contact_email, incidentId string) error {
	// create it
	var betterStackAck struct {
		AckedBy string `json:"acknowledged_by,omitempty"`
	}

	betterStackAck.AckedBy = contact_email

	if contact_email == "" {
		betterStackAck.AckedBy = default_contact_email
	}

	jsonBody, err := json.Marshal(betterStackAck)
	if err != nil {
		return err
	}
	jsonBodyReader := bytes.NewReader(jsonBody)

	req, err := b.NewRequest("POST", "/api/v2/incidents/"+incidentId+"/acknowledge", jsonBodyReader)

	res, err := b.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// check response
	if res.StatusCode != 409 && res.StatusCode != 200 {
		return fmt.Errorf("response status code was %d", res.StatusCode)
	}

	// return success
	return nil
}

func (b *BetterStackClient) ResolveIncident(contact_email, default_contact_email, incidentId string) error {
	// create it
	var betterStackAck struct {
		ResolvedBy string `json:"resolved_by"`
	}

	betterStackAck.ResolvedBy = contact_email

	if contact_email == "" {
		betterStackAck.ResolvedBy = default_contact_email
	}

	jsonBody, err := json.Marshal(betterStackAck)
	if err != nil {
		return err
	}
	jsonBodyReader := bytes.NewReader(jsonBody)

	req, err := b.NewRequest("POST", "/api/v2/incidents/"+incidentId+"/resolve", jsonBodyReader)

	res, err := b.Do(req)
	if err != nil {
		return err
	}
	fmt.Println(res, err)
	defer res.Body.Close()

	// check response
	if res.StatusCode != 409 && res.StatusCode != 200 {
		return fmt.Errorf("response status code was %d", res.StatusCode)
	}

	// return success
	return nil
}

func (b *BetterStackClient) CheckIncidentsEndpoint() error {
	req, err := b.NewRequest("GET", "/api/v2/incidents", nil)

	res, err := b.Do(req)
	fmt.Println(res, err)
	if err != nil {
		return fmt.Errorf("Failed to request /api/v2/incidents: %s", err.Error())
	}
	defer res.Body.Close()

	// check response
	if res.StatusCode != 200 {
		return fmt.Errorf("Response status code for /api/v2/incidents was %d, not 200", res.StatusCode)
	}
	return nil
}

func (b *BetterStackClient) TestGetIncident() error {
	req, err := b.NewRequest("GET", "/api/v2/incidents/697108568", nil)

	res, err := b.Do(req)
	fmt.Println(res, err)
	if err != nil {
		return fmt.Errorf("Failed to request /api/v2/incidents: %s", err.Error())
	}
	defer res.Body.Close()

	// check response
	if res.StatusCode != 200 {
		return fmt.Errorf("Response status code for /api/v2/incidents was %d, not 200", res.StatusCode)
	}
	return nil
}
