# Go parameters
GOCMD=go
SHRINKFLAGS=-ldflags="-s -w"
DEBUGTAGS=-tags DEBUG
GOBUILD=$(GOCMD) build $(SHRINKFLAGS)
GOBUILDDEBUG=$(GOCMD) build $(DEBUGTAGS)
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=server
DEBUG_BINARY_NAME=debug_$(BINARY_NAME)


prod:
	$(GOBUILD) -o $(BINARY_NAME) -v
	strip $(BINARY_NAME)
	upx --brute $(BINARY_NAME)
debug:
	$(GOBUILDDEBUG) -o $(DEBUG_BINARY_NAME) -v
clean: 
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(DEBUG_BINARY_NAME)
