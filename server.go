package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

const RplWelcome = 1
const RplListStart = 321
const RplList = 322
const RplListEnd = 323
const RplTopic = 332
const RplNamReply = 353
const RplEndOfNames = 366
const ErrNoSuchChannel = 403
const ErrNoNicknameGiven = 431
const ErrNicknameInUse = 433
const ErrNotOnChannel = 442
const ErrNotRegistered = 451
const ErrNeedMoreParams = 461

type serverInfo struct {
	clients  map[string]*clientInfo
	channels map[string]*Channel
}

type clientInfo struct {
	Conn chan []byte

	NICK     string
	RealName string

	Channels map[string]*Channel

	registered bool
}

type Channel struct {
	Name        string
	Topic       string
	Members     map[string]*clientInfo
	CreatorNick string
}

var serverState = &serverInfo{
	clients:  make(map[string]*clientInfo),
	channels: make(map[string]*Channel),
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	scanner.Scan()
	firstLine := scanner.Text()

	connectionType := getConnectionType(firstLine)

	switch connectionType {
	case "client":
		handleClient(firstLine, scanner, conn)
	case "server":
		handleServer(firstLine, scanner, conn)
	}
}

func handleClient(firstLine string, scanner *bufio.Scanner, conn net.Conn) {
	client := &clientInfo{
		Conn:     make(chan []byte),
		Channels: make(map[string]*Channel),
	}

	go forwardChannelToConn(client.Conn, conn)

	handleClientLine(client, firstLine)

	for scanner.Scan() {
		line := scanner.Text()

		fmt.Println("recv:", line)

		handleClientLine(client, line)
	}

	removeClient(client)
}

func forwardChannelToConn(channel chan []byte, conn net.Conn) {
	for {
		bytes := <-channel
		conn.Write(bytes)
	}
}

func handleServer(firstLine string, scanner *bufio.Scanner, conn net.Conn) {

}

func handleQuit(client *clientInfo, words []string) {
	if len(words) > 1 {
		reply(client, "Quit - too many parameters")
		return
	}

	for _, channel := range client.Channels {
		delete(channel.Members, client.NICK)
		if len(channel.Members) == 0 {
			delete(serverState.channels, channel.Name)
		} else {
			if channel.CreatorNick == client.NICK {
				channel.CreatorNick = ""
			}
		}
	}

	delete(serverState.clients, client.NICK)
	reply(client, "Goodbye!")
}

func handleTopic(client *clientInfo, words []string) {
	if len(words) < 2 {
		numericReply(client, ErrNeedMoreParams, "TOPIC :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		numericReply(client, ErrNoSuchChannel, channelName+" :Missing prefix")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		numericReply(client, ErrNoSuchChannel, channelName+" :Nonexistant channel")
		return
	}

	if _, exists := channel.Members[client.NICK]; !exists {
		numericReply(client, ErrNotOnChannel, channelName+" :Not on channel")
		return
	}

	if client.NICK == channel.CreatorNick {
		topic := strings.Join(words[2:], " ")
		channel.Topic = topic
		reply(client, "Updated Channel Topic")
	} else {
		numericReply(client, RplTopic, channelName+" :"+channel.Topic)
		// TODO RPL_TOPICWHOTIME
	}
}

func handleList(client *clientInfo, words []string) {
	if len(words) > 1 {
		reply(client, "List - too many parameters")
		return
	}

	numericReply(client, RplListStart, "Channel :Users Name")

	for channelName, channel := range serverState.channels {
		memberCount := len(channel.Members)
		numericReply(
			client,
			RplList,
			fmt.Sprintf("%s %d :%s", channelName, memberCount, channel.Topic),
		)
	}

	numericReply(client, RplListEnd, ":End of /LIST")
}

func handlePart(client *clientInfo, words []string) {
	if len(words) < 2 {
		numericReply(client, ErrNeedMoreParams, "PART :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		numericReply(client, ErrNoSuchChannel, channelName+" :Missing prefix")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		numericReply(client, ErrNoSuchChannel, channelName+" :Nonexistant channel")
		return
	}

	if _, exists := channel.Members[client.NICK]; !exists {
		numericReply(client, ErrNotOnChannel, channelName+" :Not on channel")
		return
	}

	delete(channel.Members, client.NICK)
	delete(client.Channels, channelName)

	if channel.CreatorNick == client.NICK {
		channel.CreatorNick = ""
	}

	if len(channel.Members) == 0 {
		delete(serverState.channels, channelName)
	}
}

func handleJoin(client *clientInfo, words []string) {
	if len(words) < 2 {
		numericReply(client, ErrNeedMoreParams, "JOIN :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		numericReply(client, ErrNoSuchChannel, channelName+" :Missing prefix")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		channel = &Channel{
			Name:        channelName,
			Members:     make(map[string]*clientInfo),
			CreatorNick: client.NICK,
		}

		serverState.channels[channelName] = channel
	}

	if _, exists := channel.Members[client.NICK]; exists {
		return
	}

	channel.Members[client.NICK] = client

	client.Channels[channelName] = channel

	reply(client, fmt.Sprintf("%s JOIN %s", client.NICK, channelName))
	numericReply(client, RplTopic, channelName+" :"+channel.Topic)

	sendNames(client, channel)
}

func handleNames(client *clientInfo, words []string) {
	if len(words) < 2 {
		numericReply(client, ErrNeedMoreParams, "NAMES :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		numericReply(client, ErrNoSuchChannel, channelName+" :Missing prefix")
		return
	}

	if channel, exists := serverState.channels[channelName]; exists {
		sendNames(client, channel)
	} else {
		numericReply(client, ErrNoSuchChannel, channelName+" :Nonexistant channel")
	}
}

func sendNames(client *clientInfo, channel *Channel) {
	var names []string

	for nick := range channel.Members {
		names = append(names, nick)
	}

	numericReply(
		client,
		RplNamReply,
		"= "+channel.Name+" :"+strings.Join(names, " "),
	)

	numericReply(client, RplEndOfNames, ":End of /NAMES list")
}

func getConnectionType(line string) string {
	words := strings.Fields(line)

	if len(words) == 0 {
		return "client"
	}

	switch words[0] {
	case "NICK":
		return "client"

	case "SERVER":
		return "server"

	default:
		return "client"
	}
}

func handleClientLine(client *clientInfo, line string) {
	words := strings.Fields(line)

	if len(words) == 0 {
		return
	}

	switch words[0] {

	case "NICK":
		handleNick(client, words)

	case "USER":
		handleUser(client, line)

	case "PING":
		if len(words) < 2 {
			reply(client, "PONG")
			return
		}
		msg := strings.Join(words[1:], " ")
		reply(client, fmt.Sprintf("PONG %s", msg))

	case "JOIN":
		if !client.registered {
			numericReply(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleJoin(client, words)

	case "PRIVMSG":
		if !client.registered {
			numericReply(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleMessage(client, words)

	case "PART":
		if !client.registered {
			numericReply(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handlePart(client, words)

	case "QUIT":
		handleQuit(client, words)

	case "NAMES":
		if !client.registered {
			numericReply(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleNames(client, words)

	case "LIST":
		if !client.registered {
			numericReply(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleList(client, words)

	case "TOPIC":
		if !client.registered {
			numericReply(client, ErrNotRegistered, ":You have not registered")
			return
		}
		handleTopic(client, words)
	}

	tryRegister(client)

}

func handleMessage(client *clientInfo, words []string) {
	if len(words) < 3 {
		numericReply(client, ErrNeedMoreParams, "PRIVMSG :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		numericReply(client, ErrNoSuchChannel, channelName+" :Missing prefix")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		numericReply(client, ErrNoSuchChannel, channelName+" :Nonexistant channel")
		return
	}

	msg := strings.Join(words[2:], " ")
	msg = strings.TrimPrefix(msg, ":")

	for _, channelClient := range channel.Members {
		if channelClient != client {
			reply(channelClient, fmt.Sprintf("%s: %s", client.NICK, msg))
		}
	}
}

func tryRegister(client *clientInfo) {
	if client.registered {
		return
	}

	if client.NICK == "" {
		return
	}

	if client.RealName == "" {
		return
	}

	client.registered = true

	numericReply(client, RplWelcome, ":Welcome to IRC")
}

// func checkRegistration(incomingConn net.Conn) bool {
// 	for _, client := range serverState.clients {
// 		if incomingConn == client.Conn {
// 			return true
// 		}
// 	}

// 	return false
// }

func handleNick(client *clientInfo, words []string) {
	if len(words) < 2 {
		numericReply(client, ErrNoNicknameGiven, ":No nickname given")
		return
	}

	newNick := words[1]

	existingClient, exists := serverState.clients[newNick]
	if exists && existingClient != client {
		numericReply(client, ErrNicknameInUse, ":Nickname is already in use")
		return
	}

	oldNick := client.NICK
	if oldNick == newNick {
		return
	}

	if oldNick != "" {
		delete(serverState.clients, oldNick)
	}

	client.NICK = newNick
	serverState.clients[newNick] = client

	for _, channel := range serverState.channels {
		if channel.CreatorNick == oldNick {
			channel.CreatorNick = newNick
		}

		if member, exists := channel.Members[oldNick]; exists {
			delete(channel.Members, oldNick)
			channel.Members[newNick] = member
		}
	}
}

func handleUser(client *clientInfo, line string) {
	split := strings.SplitN(line, ":", 2)

	if len(split) < 2 {
		return
	}

	client.RealName = split[1]
	// TODO this breaks it. why.
	// client.registered = true
}

func numericReply(client *clientInfo, num uint16, msg string) {
	reply(client, fmt.Sprintf("%03d %s %s", num, client.NICK, msg))
}

func reply(client *clientInfo, msg string) {
	fmt.Println("send:", msg)

	client.Conn <- ([]byte(msg + "\r\n"))
}

func removeClient(client *clientInfo) {
	for channelName, channel := range client.Channels {
		delete(channel.Members, client.NICK)

		if len(channel.Members) == 0 {
			delete(serverState.channels, channelName)
		}
	}

	if client.NICK != "" {
		delete(serverState.clients, client.NICK)
	}

	fmt.Println("client disconnected:", client.NICK)
}

func runServer() {
	ln, err := net.Listen("tcp", ":6667")
	fmt.Println("Server listening on", ln.Addr())
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handleConnection(conn)
	}
}
