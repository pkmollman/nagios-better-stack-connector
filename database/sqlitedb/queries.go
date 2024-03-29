package sqlitedb

// type EventItem struct {
// 	Id                       string `json:"id"`
// 	NagiosSiteName           string `json:"nagiosSiteName"`
// 	NagiosProblemId          int    `json:"nagiosProblemId"`
// 	NagiosProblemType        string `json:"nagiosProblemType"`
// 	NagiosProblemHostname    string `json:"nagiosProblemHostname"`
// 	NagiosProblemServiceName string `json:"nagiosProblemServiceName"`
// 	NagiosProblemContent     string `json:"nagiosProblemContent"`
// 	// ("PROBLEM", "RECOVERY", "ACKNOWLEDGEMENT", "FLAPPINGSTART", "FLAPPINGSTOP", "FLAPPINGDISABLED", "DOWNTIMESTART", "DOWNTIMEEND", "DOWNTIMECANCELLED")
// 	NagiosProblemNotificationType string `json:"nagiosProblemNotificationType"`
// 	BetterStackIncidentId         string `json:"betterStackIncidentId"`
// }

var CreateEventItemTable = `CREATE TABLE IF NOT EXISTS event (
	id INTEGER PRIMARY KEY,
	nagiosSiteName TEXT,
	nagiosProblemId INTEGER,
	nagiosProblemType TEXT,
	nagiosProblemHostname TEXT,
	nagiosProblemServiceName TEXT,
	nagiosProblemContent TEXT,
	nagiosProblemNotificationType TEXT,
	betterStackIncidentId TEXT )`

var GetAllEventItemsQuery = `SELECT
	id,
	nagiosSiteName,
	nagiosProblemId,
	nagiosProblemType,
	nagiosProblemHostname,
	nagiosProblemServiceName,
	nagiosProblemContent,
	nagiosProblemNotificationType,
	betterStackIncidentId
	FROM event`

var CreateEventItemQuery = `INSERT INTO event (
	nagiosSiteName,
	nagiosProblemId,
	nagiosProblemType,
	nagiosProblemHostname,
	nagiosProblemServiceName,
	nagiosProblemContent,
	nagiosProblemNotificationType,
	betterStackIncidentId ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

var InsertEventItemQuery = `INSERT INTO event (
	nagiosSiteName,
	nagiosProblemId,
	nagiosProblemType,
	nagiosProblemHostname,
	nagiosProblemServiceName,
	nagiosProblemContent,
	nagiosProblemNotificationType,
	betterStackIncidentId ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
