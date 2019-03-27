package model

import (
	"go-implant/common/communication"

	"github.com/patrickmn/go-cache"
)

var db *cache.Cache

// InitDB inits the database access
func InitDB() {

	// Create a cache with no expiration at all, and which
	// never automatically purges expired items
	db = cache.New(cache.NoExpiration, cache.NoExpiration)
}

// Exists checks if UID exists
func Exists(UID string) bool {
	_, found := db.Get(UID)
	return found
}

// Fetch fetches the client from database. returns nil if not found - panics if not found!
func Fetch(UID string) communication.Client {
	client, _ := db.Get(UID)
	return client.(communication.Client)
}

// Items returns map of all clients
func Items() map[string]communication.Client {
	clientmap := map[string]communication.Client{}
	interfacemap := db.Items()
	for key, value := range interfacemap {
		clientmap[key] = value.Object.(communication.Client)
	}
	return clientmap
}

// Store stores a client structure to database
func Store(UID string, client communication.Client) {
	db.Set(UID, client, cache.NoExpiration)
}

// Remove removes a client from database
func Remove(UID string) {
	db.Delete(UID)
}
