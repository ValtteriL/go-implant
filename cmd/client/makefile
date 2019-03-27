# Go parameters
GOCMD=go
SHRINKFLAGS=-ldflags="-s -w"
DEBUGTAGS=-tags DEBUG
GOBUILD=$(GOCMD) build $(SHRINKFLAGS)
GOBUILDDEBUG=$(GOCMD) build $(DEBUGTAGS)
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=client
BINARY_LINUX=$(BINARY_NAME)_linux
BINARY_WIN=$(BINARY_NAME)_win.exe
BINARY_MAC=$(BINARY_NAME)_mac
DEBUG_BINARY_LINUX=debug_$(BINARY_NAME)_linux
DEBUG_BINARY_WIN=debug_$(BINARY_NAME)_win.exe
DEBUG_BINARY_MAC=debug_$(BINARY_NAME)_mac

prod:
	make build-linux
	make build-windows
	make build-mac
debug:
	make build-linux-debug
	make build-windows-debug
	make build-mac-debug
clean: 
	$(GOCLEAN)
	rm -f $(BINARY_LINUX)
	rm -f $(BINARY_WIN)
	rm -f $(BINARY_MAC)
	rm -f $(DEBUG_BINARY_LINUX)
	rm -f $(DEBUG_BINARY_WIN)
	rm -f $(DEBUG_BINARY_MAC)

# build all platforms
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_LINUX) -v
	strip $(BINARY_LINUX)
	upx --brute $(BINARY_LINUX)
build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_WIN) -v
	strip $(BINARY_WIN)
	upx --brute $(BINARY_WIN)
build-mac:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_MAC) -v
	upx --brute $(BINARY_MAC)

# build debug all platforms
build-linux-debug:
	GOOS=linux GOARCH=amd64 $(GOBUILDDEBUG) -o $(DEBUG_BINARY_LINUX) -v
build-windows-debug:
	GOOS=windows GOARCH=amd64 $(GOBUILDDEBUG) -o $(DEBUG_BINARY_WIN) -v
build-mac-debug:
	GOOS=darwin GOARCH=amd64 $(GOBUILDDEBUG) -o $(DEBUG_BINARY_MAC) -v