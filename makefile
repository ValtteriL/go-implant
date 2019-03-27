# Go parameters
GOCMD=go
GOGET=$(GOCMD) get

# fetch dependencies
deps:
	$(GOGET) github.com/denisbrodbeck/machineid
	$(GOGET) golang.org/x/sys/windows/registry
	$(GOGET) github.com/kr/pty
	$(GOGET) github.com/znly/strobfus
