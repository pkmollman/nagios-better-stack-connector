package nagios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type HostState struct {
	DisplayName string `json:"display_name"`
	// 0 not Acknowledged, 1 Acknowledged
	Acknowledged int      `json:"acknowledged"`
	State        int      `json:"state"`
	IpAddr       string   `json:"address"`
	Services     []string `json:"services"`
}

func (n *NagiosClient) GetHosts() ([]HostState, error) {
	req, err := n.NewRequest("GET", fmt.Sprintf("/%s/thruk/r/hosts", n.siteName), nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Nagios returned status code %d instead of %d", res.StatusCode, http.StatusOK)
	}

	hosts := []HostState{}
	err = json.NewDecoder(res.Body).Decode(&hosts)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func (n *NagiosClient) GetHostState(host string) (HostState, error) {
	host = url.QueryEscape(host)
	req, err := n.NewRequest("GET", fmt.Sprintf("/%s/thruk/r/hosts?name=%s", n.siteName, host), nil)
	if err != nil {
		return HostState{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return HostState{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return HostState{}, fmt.Errorf("Nagios returned status code %d instead of %d", res.StatusCode, http.StatusOK)
	}

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
