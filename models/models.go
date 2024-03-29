package models

type EventItem struct {
	Id                       string `json:"id"`
	NagiosSiteName           string `json:"nagiosSiteName"`
	NagiosProblemId          int    `json:"nagiosProblemId"`
	NagiosProblemType        string `json:"nagiosProblemType"`
	NagiosProblemHostname    string `json:"nagiosProblemHostname"`
	NagiosProblemServiceName string `json:"nagiosProblemServiceName"`
	NagiosProblemContent     string `json:"nagiosProblemContent"`
	// ("PROBLEM", "RECOVERY", "ACKNOWLEDGEMENT", "FLAPPINGSTART", "FLAPPINGSTOP", "FLAPPINGDISABLED", "DOWNTIMESTART", "DOWNTIMEEND", "DOWNTIMECANCELLED")
	NagiosProblemNotificationType string `json:"nagiosProblemNotificationType"`
	BetterStackPolicyId           string `json:"betterStackPolicyId"`
	BetterStackIncidentId         string `json:"betterStackIncidentId"`
	InteractingUserEmail          string `json:"interactingUserEmail"`
}

// json example:
// {
// 	"nagiosSiteName": "telops",
// 	"nagiosProblemHostname": "test-host",
// 	"nagiosProblemServiceName": "test-service",
// 	"nagiosProblemContent": "some problem",
// 	"nagiosProblemNotificationType": "PROBLEM",
// 	"betterStackPolicyId": "some-policy-id",
// 	"nagiosProblemId": 23123,
// }
