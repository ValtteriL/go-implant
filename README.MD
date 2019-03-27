# Go-Implant
Go-Implant is a cross-platform post-exploitation agent written in Go. It consists of a server and a client (or agent). The agent beacons to the server periodically using http POST requests. As a reply to the beacon, the server returns list of commands to execute on the client. Built-in commands include port forwards, reverse shell and configuration changes.

To make detection and forensics harder:
- All communications in Go-Implant are encrypted and look 'normal' in traffic analysis. Beacons by default use HTTPS, while reverse shells and port forwards use SSH tunnels.
- The http server looks like a default installation of nginx to others than the agent.
- All sensitive strings inside the agent binary, such as configuration, are AES encrypted using random key.
- Agent configuration is flexible, user can choose for example beacon interval, user agent of beacons and beacon endpoints. 


### Note
Go-Implant is not a tool for committing crime. There is no functionality for criminal activities inbuilt to it, nor will it be added. This means for example that there is no persistence or modules for stealing credentials from systems.
The Author is not responsible of anything you use go-implant for.


## Why?
Because no long-deployable, cross-platform, flexible reverse shell with only basic functionality and standard protocols was found. Go-Implant is an attempt to create one.


## Features
- Encrypted communications
- Reverse shell access
- upload / download files
- Reverse port forwarding


## Installation
`apt install -y git make upx strip`
`git clone <repo name>`
`cd <repo name>`
`make deps`


## Compilation
Both client and server of Go-Implant are standalone executables that need to be compiled. Server has only been tested to compile and work on linux. Client has been tested to compile on Linux, and work on Linux and Windows.

### Server
1. Compile the server: (in cmd/server)
    `make prod`

### Client
1. Set configuration in /client/config/parameters.go
2. Encrypt static strings
    `cd client/config/`
    remove //build +ignore tag from parameters.go
    `go generate`
    add //build +ignore tag back to parameters.go
3. Compile the client for all platforms (in cmd/client)
    `make prod`

The makefile compiles the client for all platforms, strips symbols from the executables and finally packs them using upx.
As of writing mac executables are not stripped.
NOTE: makefile assumes commands upx and strip are available

### Debug builds
Should anything not work as expected, debugging builds are available of both server and client. These build include debugging symbols, a ton of printouts to tracks the activities of the program and omit compressing the resulting binaries. Debugging builds are trivial to reverse-engineer and therefore should not be used in production.

To compile debugging build, use command:
`make debug`
for both client and the server.

#### Configuration
The following parameters are freely configurable by the user in /client/config/parameters.go:

Sleeptime (int) - the time slept between beacons in seconds
Jitter (int) - random extra delay to be added to the sleeptime
Retries (int) - amount of tries to take in case C2 is unreachable
UserAgent (string) - the user agent that gets sent on beacons
CCHost (string) - the base server url
Endpoints (string[]) - the endpoints in the server to which beacon. They are chosen randomly


## Usage

### Server

You should get a SSL certificate and corresponding private key from Lets encrypt for best results. The agent will not trust self-signed certificates unless the root certificate has been installed first.

```
Usage of ./server:
  -cert string
    	file with https cert (default "server.crt")
  -forwarded-timeout duration
    	forwarded-tcpip timeout (default 3h0m0s)
  -handlerport int
    	port of HTTP* listener (default 443)
  -http
    	use HTTP instead of HTTPS
  -main-timeout duration
    	Client socket timeout (default 3h0m0s)
  -privkey string
    	file with private key for cert (default "server.key")
  -sshport int
    	port of SSH listener (default 22)
```

#### examples
`./server -handlerport 8080 -sshport 2222`
`./server -cert othercert.crt -privkey otherkey.key`
`./server -forwarded-timeout 0h10m0s`


## Reverse shell and file access

Reverse shell access works as follows: The client starts a sshd listening on localhost and then initializes a reverse port forward to the server. The sshd on the client can then be accessed from the server using the forwarded port. There are commands quickssh and serveSSH on the server that start the reverse shell. quickssh uses the inbuilt sshd on the Go-Implant server to catch the port forward, serveSSH enables the usage any normal sshd for the purpose. 

NOTE: in order for the client to be able to forward port to the host, it needs to have valid SSH credentials. One must make sure that the credentials cannot be used to obtain a shell to the server when using normal sshd. In the inbuilt SSH, only reverse port forwards are allowed by clients.

Filesystem access is done using sftp.

### Unix (linux or mac):
After forwarding port from the client to server, just SSH in using the generated credentials.
`ssh <username>@localhost -p <port>`

### Windows:
After forwarding port from the client to server, forward another port from the client machine using the client's sshd to localhost, then connect to the forwarded port using netcat
`ssh -N -L <localport>:0.0.0.0:<destport> localhost -p <port>`
`nc localhost <localport>`

NOTE: right now destport is not used! The remote host 0.0.0.0 is a magic string that tells the agent to serve a cmd shell to the client.

### Uploading / Downloading files with sftp (both windows and unix)
After forwarding port from the client to server, you can access the filesystem using sftp.
`sftp -P <port> <username>@localhost`


## Port forwarding

Local ssh listener in the server allows at most 5 reverse port forwards from the victim.

### Windows:
On windows, ports can be forwarded but no shell can be launched at the same time, therefore flag -N must be used. Omitting the flag causes the client to serve sftp instead
`ssh -N -L <localport>:<desthost>:<destport> localhost -p <port>`

### Unix (linux or mac):
On linux, ports can be forwarded similarly with the familiar sshd
`ssh [-N] -L <localport>:<desthost>:<destport> localhost -p <port>`


## Features for future releases
- add x11 forwarding to unix ssh handler
- encrypt protocol strings


## Contributing
Contributions are more than welcome! Please submit your well tested code as a pull request. Writing documentation is also encouraged.
