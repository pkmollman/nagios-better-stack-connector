package nagios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	// marshal struct to json to reader
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

	// // print response body
	// b, err := io.ReadAll(res.Body)
	// if err != nil {
	// 	return err
	// }

	// fmt.Println(string(b))

	return nil
}
