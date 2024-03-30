package nagios

import (
	"io"
	"net/http"
)

type NagiosClient struct {
	apiUser  string
	apiKey   string
	baseUrl  string
	siteName string
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
