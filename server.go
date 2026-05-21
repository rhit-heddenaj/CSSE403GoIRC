package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

const (
	ServerName          = "localhost"
	RplWelcome          = 1
	RplListStart        = 321
	RplList             = 322
	RplListEnd          = 323
	RplNoTopic          = 331
	RplTopic            = 332
	RplNameReply        = 353
	RplEndOfNames       = 366
	ErrNoSuchChan       = 403
	ErrCannotSendToChan = 404
	ErrUnknownCommand   = 421
	ErrNotOnChan        = 442
	ErrNotRegistered    = 451
	ErrNeedMoreParams   = 461
	ErrNickInUse        = 433
	ErrBadChanMask      = 479
)

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
	quitMsg := ""
	if len(words) > 1 {
		quitMsg = strings.TrimPrefix(strings.Join(words[1:], " "), ":")
	}

	for _, channel := range client.Channels {
		msg := fmt.Sprintf(":%s QUIT", client.NICK)
		if quitMsg != "" {
			msg += " :" + quitMsg
		}
		msg += "\r\n"
		for _, member := range channel.Members {
			if member != client {
				reply(member, msg)
			}
		}

		delete(channel.Members, client.NICK)
		if len(channel.Members) == 0 {
			delete(serverState.channels, channel.Name)
		} else if channel.CreatorNick == client.NICK {
			channel.CreatorNick = ""
		}
	}

	delete(serverState.clients, client.NICK)
}

func handleTopic(client *clientInfo, words []string) {
	if len(words) < 2 {
		replyNumeric(client, ErrNeedMoreParams, "TOPIC :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		replyNumeric(client, ErrBadChanMask, fmt.Sprintf("%s :Bad Channel Mask", channelName))
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		replyNumeric(client, ErrNoSuchChan, fmt.Sprintf("%s :No such channel", channelName))
		return
	}

	if _, exists := channel.Members[client.NICK]; !exists {
		replyNumeric(client, ErrNotOnChan, fmt.Sprintf("%s :You're not on that channel", channelName))
		return
	}

	if len(words) >= 3 {
		topic := strings.TrimPrefix(strings.Join(words[2:], " "), ":")
		channel.Topic = topic
		for _, member := range channel.Members {
			reply(member, fmt.Sprintf(":%s TOPIC %s :%s\r\n", client.NICK, channelName, channel.Topic))
		}
		return
	}

	sendTopic(client, channel)
}

func handleList(client *clientInfo, words []string) {
	replyNumeric(client, RplListStart, "Channel :Users Name")

	for channelName, channel := range serverState.channels {
		memberCount := len(channel.Members)
		replyNumeric(client, RplList, fmt.Sprintf("%s %d :%s", channelName, memberCount, channel.Topic))
	}

	replyNumeric(client, RplListEnd, ":End of /LIST")
}

func handlePart(client *clientInfo, words []string) {
	if len(words) < 2 {
		replyNumeric(client, ErrNeedMoreParams, "PART :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		replyNumeric(client, ErrBadChanMask, fmt.Sprintf("%s :Bad Channel Mask", channelName))
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		replyNumeric(client, ErrNoSuchChan, fmt.Sprintf("%s :No such channel", channelName))
		return
	}

	if _, exists := channel.Members[client.NICK]; !exists {
		replyNumeric(client, ErrNotOnChan, fmt.Sprintf("%s :You're not on that channel", channelName))
		return
	}

	partMsg := ""
	if len(words) > 2 {
		partMsg = strings.TrimPrefix(strings.Join(words[2:], " "), ":")
	}
	msg := fmt.Sprintf(":%s PART %s", client.NICK, channelName)
	if partMsg != "" {
		msg += " :" + partMsg
	}
	msg += "\r\n"
	for _, member := range channel.Members {
		reply(member, msg)
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
		replyNumeric(client, ErrNeedMoreParams, "JOIN :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		replyNumeric(client, ErrBadChanMask, fmt.Sprintf("%s :Bad Channel Mask", channelName))
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

	joinMsg := fmt.Sprintf(":%s JOIN %s\r\n", client.NICK, channelName)
	for _, member := range channel.Members {
		reply(member, joinMsg)
	}

	sendTopic(client, channel)
	sendNames(client, channel)
}

func handleNames(client *clientInfo, words []string) {
	if len(words) < 2 {
		replyNumeric(client, ErrNeedMoreParams, "NAMES :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		replyNumeric(client, ErrBadChanMask, fmt.Sprintf("%s :Bad Channel Mask", channelName))
		return
	}

	if channel, exists := serverState.channels[channelName]; exists {
		sendNames(client, channel)
		return
	}

	replyNumeric(client, ErrNoSuchChan, fmt.Sprintf("%s :No such channel", channelName))
}

func sendNames(client *clientInfo, channel *Channel) {
	var names []string

	for nick := range channel.Members {
		names = append(names, nick)
	}

	replyNumeric(client, RplNameReply, fmt.Sprintf("= %s :%s", channel.Name, strings.Join(names, " ")))
	replyNumeric(client, RplEndOfNames, fmt.Sprintf("%s :End of /NAMES list", channel.Name))
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
			reply(client, "PONG\r\n")
			return
		}
		msg := strings.Join(words[1:], " ")
		reply(client, fmt.Sprintf("PONG %s\r\n", msg))

	case "CAP":
		if len(words) >= 2 {
			switch words[1] {
			case "LS":
				reply(client, fmt.Sprintf(":%s CAP * LS :\r\n", ServerName))
			case "REQ":
				req := ""
				if len(words) > 2 {
					req = strings.TrimPrefix(strings.Join(words[2:], " "), ":")
				}
				reply(client, fmt.Sprintf(":%s CAP * NAK :%s\r\n", ServerName, req))
			case "END":
				// ignore, not implemented
			}
		}

	case "JOIN":
		if !client.registered {
			replyNumeric(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleJoin(client, words)

	case "PRIVMSG":
		if !client.registered {
			replyNumeric(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleMessage(client, words)

	case "PART":
		if !client.registered {
			replyNumeric(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handlePart(client, words)

	case "QUIT":
		handleQuit(client, words)

	case "NAMES":
		if !client.registered {
			replyNumeric(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleNames(client, words)

	case "LIST":
		if !client.registered {
			replyNumeric(client, ErrNotRegistered, ":You have not registered")
			return
		}

		handleList(client, words)

	case "TOPIC":
		if !client.registered {
			replyNumeric(client, ErrNotRegistered, ":You have not registered")
			return
		}
		handleTopic(client, words)

	default:
		replyNumeric(client, ErrUnknownCommand, fmt.Sprintf("%s :Unknown command", words[0]))
	}

	tryRegister(client)

}

func handleMessage(client *clientInfo, words []string) {
	if len(words) < 3 {
		replyNumeric(client, ErrNeedMoreParams, "PRIVMSG :Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		replyNumeric(client, ErrBadChanMask, fmt.Sprintf("%s :Bad Channel Mask", channelName))
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		replyNumeric(client, ErrNoSuchChan, fmt.Sprintf("%s :No such channel", channelName))
		return
	}

	if _, exists := channel.Members[client.NICK]; !exists {
		replyNumeric(client, ErrCannotSendToChan, fmt.Sprintf("%s :Cannot send to channel", channelName))
		return
	}

	msg := strings.Join(words[2:], " ")
	msg = strings.TrimPrefix(msg, ":")

	for _, channelClient := range channel.Members {
		if channelClient != client {
			reply(channelClient, fmt.Sprintf(":%s PRIVMSG %s :%s\r\n", client.NICK, channelName, msg))
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

	replyNumeric(client, RplWelcome, ":Welcome to IRC")
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
		replyNumeric(client, ErrNeedMoreParams, "NICK :Not enough parameters")
		return
	}

	newNick := words[1]

	existingClient, exists := serverState.clients[newNick]
	if exists && existingClient != client {
		replyNumeric(client, ErrNickInUse, fmt.Sprintf("%s :Nickname is already in use", newNick))
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

	if oldNick == "" {
		return
	}

	notifyTargets := make(map[*clientInfo]struct{})
	for _, channel := range serverState.channels {
		if channel.CreatorNick == oldNick {
			channel.CreatorNick = newNick
		}

		if member, exists := channel.Members[oldNick]; exists {
			delete(channel.Members, oldNick)
			channel.Members[newNick] = member
		}

		for _, member := range channel.Members {
			notifyTargets[member] = struct{}{}
		}
	}

	nickMsg := fmt.Sprintf(":%s NICK :%s\r\n", oldNick, newNick)
	for member := range notifyTargets {
		reply(member, nickMsg)
	}
}

func handleUser(client *clientInfo, line string) {
	split := strings.SplitN(line, ":", 2)

	if len(split) < 2 {
		replyNumeric(client, ErrNeedMoreParams, "USER :Not enough parameters")
		return
	}

	client.RealName = split[1]
}

func reply(client *clientInfo, msg string) {
	fmt.Println("send:", msg)

	client.Conn <- ([]byte(msg))
}

func replyNumeric(client *clientInfo, code int, payload string) {
	target := client.NICK
	if target == "" {
		target = "*"
	}

	if payload == "" {
		reply(client, fmt.Sprintf(":%s %03d %s\r\n", ServerName, code, target))
		return
	}

	reply(client, fmt.Sprintf(":%s %03d %s %s\r\n", ServerName, code, target, payload))
}

func sendTopic(client *clientInfo, channel *Channel) {
	if channel.Topic == "" {
		replyNumeric(client, RplNoTopic, fmt.Sprintf("%s :No topic is set", channel.Name))
		return
	}

	replyNumeric(client, RplTopic, fmt.Sprintf("%s :%s", channel.Name, channel.Topic))
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
