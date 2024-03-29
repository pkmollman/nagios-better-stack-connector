package nagios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type NagiosClient struct {
	apiUser  string
	apiKey   string
	baseUrl  string
	siteName string
}

type NagiosHost struct {
	Name string `json:"display_name"`
}

func NewNagiosClient(apiUser, apiKey, baseUrl, siteName string) *NagiosClient {
	return &NagiosClient{
		apiKey:   apiKey,
		apiUser:  apiUser,
		baseUrl:  baseUrl,
		siteName: siteName,
	}
}

func (n *NagiosClient) NewRequest(httpMethod, endpoint string, data io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(httpMethod, n.baseUrl+endpoint, data)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Thruk-Auth-Key", n.apiKey)
	req.Header.Set("X-Thruk-Auth-User", n.apiUser)

	return req, nil
}

func (n *NagiosClient) GetHosts() ([]NagiosHost, error) {
	req, err := n.NewRequest("GET", fmt.Sprintf("/%s/thruk/r/hosts", n.siteName), nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	var hosts []NagiosHost
	err = json.NewDecoder(res.Body).Decode(&hosts)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func (n *NagiosClient) AckService(host, service, comment string) error {
	commandMap := map[string]string{
		"cmd":          "acknowledge_svc_problem",
		"host":         host,
		"service":      service,
		"comment_data": comment,
	}

	jsonBody, err := json.Marshal(commandMap)
	if err != nil {
		return err
	}

	req, err := n.NewRequest("POST", fmt.Sprintf("/%s/thruk/r/cmd", n.siteName), bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	return nil
}

type ServiceState struct {
	Acknowledged int    `json:"acknowledged"`
	State        int    `json:"state"`
	CheckOutput  string `json:"plugin_output"`
	HostAddress  string `json:"host_address"`
	HostName     string `json:"host_name"`
}

func (n *NagiosClient) GetServiceState(host, service string) (ServiceState, error) {
	// url encode host and service

	host = url.QueryEscape(host)
	service = url.QueryEscape(service)
	req, err := n.NewRequest("GET", fmt.Sprintf("/%s/thruk/r/services/%s/%s", n.siteName, host, service), nil)
	if err != nil {
		return ServiceState{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ServiceState{}, err
	}
	defer res.Body.Close()

	// bodyBytes, err := io.ReadAll(res.Body)
	// println(string(bodyBytes))

	var serviceStateResponse []ServiceState
	err = json.NewDecoder(res.Body).Decode(&serviceStateResponse)
	if err != nil {
		return ServiceState{}, err
	}

	if len(serviceStateResponse) != 1 {
		return ServiceState{}, fmt.Errorf("failed to get service state")
	}

	return serviceStateResponse[0], nil
}

type HostState struct {
	Acknowledged int    `json:"acknowledged"`
	State        int    `json:"state"`
	IpAddr       string `json:"address"`
}

func (n *NagiosClient) GetHostState(host string) (HostState, error) {
	host = url.QueryEscape(host)
	req, err := n.NewRequest("GET", fmt.Sprintf("/%s/thruk/r/hosts/%s", n.siteName, host), nil)
	if err != nil {
		return HostState{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return HostState{}, err
	}
	defer res.Body.Close()

	// print body
	// bodyBytes, err := io.ReadAll(res.Body)
	// println(string(bodyBytes))

	var hostStateResponse []HostState
	err = json.NewDecoder(res.Body).Decode(&hostStateResponse)
	if err != nil {
		return HostState{}, err
	}

	if len(hostStateResponse) != 1 {
		return HostState{}, fmt.Errorf("failed to get service state")
	}

	return hostStateResponse[0], nil
}

func (n *NagiosClient) AckHost(host, comment string) error {
	commandMap := map[string]string{
		"comment_data": comment,
	}

	jsonBody, err := json.Marshal(commandMap)
	if err != nil {
		return err
	}

	host = url.QueryEscape(host)

	req, err := n.NewRequest("POST", fmt.Sprintf("/%s/thruk/r/hosts/%s/cmd/acknowledge_host_problem", n.siteName, host), bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	return nil
}
