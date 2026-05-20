package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

const RplWelcome = 1

type serverInfo struct {
	clients  map[string]*clientInfo
	channels map[string]*Channel
}

type clientInfo struct {
	Conn net.Conn

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

	client := &clientInfo{
		Conn:     conn,
		Channels: make(map[string]*Channel),
	}

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := scanner.Text()

		fmt.Println("recv:", line)

		handleLine(client, line)
	}

	removeClient(client)
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
		reply(client, "Topic - Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		reply(client, "Invalid channel name")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		reply(client, "Channel does not exist")
		return
	}

	if _, exists := channel.Members[client.NICK]; !exists {
		reply(client, "Can't view topic of a channel you are not a part of")
		return
	}

	if client.NICK == channel.CreatorNick {
		topic := strings.Join(words[2:], " ")
		channel.Topic = topic
		reply(client, "Updated Channel Topic")
	} else {
		reply(client, "TOPIC: "+channel.Topic)
	}
}

func handleList(client *clientInfo, words []string) {
	if len(words) > 1 {
		reply(client, "List - too many parameters")
		return
	}

	reply(client, "Channel List:")

	for channelName, channel := range serverState.channels {
		memberCount := len(channel.Members)
		reply(client, fmt.Sprintf("%s (%d members)", channelName, memberCount))
	}

	reply(client, "End of list")
}

func handlePart(client *clientInfo, words []string) {
	if len(words) < 2 {
		reply(client, "Part - Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		reply(client, "Invalid channel name")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		reply(client, "Channel does not exist")
		return
	}

	if _, exists := channel.Members[client.NICK]; !exists {
		reply(client, "Can't leave a channel you are not a part of")
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
		reply(client, "JOIN - Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		reply(client, "Invalid channel name")
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

	reply(client,
		fmt.Sprintf("%s JOIN %s",
			client.NICK,
			channelName))

	sendNames(client, channel)
}

func handleNames(client *clientInfo, words []string) {
	if len(words) < 2 {
		reply(client, "Names - Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		reply(client, "Invalid channel name")
		return
	}

	if channel, exists := serverState.channels[channelName]; exists {
		sendNames(client, channel)
		return
	}

	reply(client, "Channel doesn't exist")

}

func sendNames(client *clientInfo, channel *Channel) {
	var names []string

	for nick := range channel.Members {
		names = append(names, nick)
	}

	reply(client,
		fmt.Sprintf("%s :%s",
			channel.Name,
			strings.Join(names, " ")))

	reply(client, "End of /NAMES list")
}

func handleLine(client *clientInfo, line string) {
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
			reply(client, "You have not registered")
			return
		}

		handleJoin(client, words)

	case "PRIVMSG":
		if !client.registered {
			reply(client, "You have not registered")
			return
		}

		handleMessage(client, words)

	case "PART":
		if !client.registered {
			reply(client, "You have not registered")
			return
		}

		handlePart(client, words)

	case "QUIT":
		handleQuit(client, words)

	case "NAMES":
		if !client.registered {
			reply(client, "You have not registered")
			return
		}

		handleNames(client, words)

	case "LIST":
		if !client.registered {
			reply(client, "You have not registered")
			return
		}

		handleList(client, words)

	case "TOPIC":
		if !client.registered {
			reply(client, "You have not registered")
			return
		}
		handleTopic(client, words)
	}

	tryRegister(client)

}

func handleMessage(client *clientInfo, words []string) {
	if len(words) < 3 {
		reply(client, "PRIVMSG - Not enough parameters")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		reply(client, "Invalid channel name")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		reply(client, "Channel doesn't exist")
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

	reply(client, "Welcome to IRC")
}

func checkRegistration(incomingConn net.Conn) bool {
	for _, client := range serverState.clients {
		if incomingConn == client.Conn {
			return true
		}
	}

	return false
}

func handleNick(client *clientInfo, words []string) {
	if len(words) < 2 {
		return
	}

	newNick := words[1]

	existingClient, exists := serverState.clients[newNick]
	if exists && existingClient != client {
		reply(client, ":Nickname already in use")
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

func reply(client *clientInfo, msg string) {
	fmt.Println("send:", msg)

	_, err := client.Conn.Write([]byte(msg + "\r\n"))
	if err != nil {
		fmt.Println("write error:", err)
	}
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

	client.Conn.Close()

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
