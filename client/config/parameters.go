// +build ignore

package config

//go:generate strobfus -filename $GOFILE

// variables that should be obfuscated in binary

// Sleeptime is the time slept between beacons in seconds
var Sleeptime = 5

// Jitter is random extra delay to be added to the sleeptime
var Jitter = 5

// Retries is the amount of tries to keep trying in case C2 is unreachable
var Retries = 3

// UserAgent is user agent that gets sent on beacons
var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.113 Safari/537.36"

// CCHost is the base server url
var CCHost = "http://127.0.0.1:8080/"

// Endpoints are the endpoints in the server to which beacon. They are chosen randomly
var Endpoints = []string{"index.php", "api/forward", "submit.php", "admin/get.php", "news.php", "login/process.php"}
