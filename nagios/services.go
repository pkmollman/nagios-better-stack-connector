package nagios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type ServiceState struct {
	DisplayName  string `json:"display_name"`
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

	req, err := n.NewRequest("GET", fmt.Sprintf("/%s/thruk/r/services?host_name=%s&description=%s", n.siteName, host, service), nil)
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
	//

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
