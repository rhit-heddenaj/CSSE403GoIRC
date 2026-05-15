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
	Name    string
	Topic   string
	Members map[string]*clientInfo
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

func handleJoin(client *clientInfo, words []string) {
	if len(words) < 2 {
		reply(client, "JOIN - Not enough parameters\r\n")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		reply(client, "Invalid channel name\r\n")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		channel = &Channel{
			Name:    channelName,
			Members: make(map[string]*clientInfo),
		}

		serverState.channels[channelName] = channel
	}

	if _, exists := channel.Members[client.NICK]; exists {
		return
	}

	channel.Members[client.NICK] = client

	client.Channels[channelName] = channel

	reply(client,
		fmt.Sprintf("%s JOIN %s\r\n",
			client.NICK,
			channelName))

	sendNames(client, channel)
}

func sendNames(client *clientInfo, channel *Channel) {
	var names []string

	for nick := range channel.Members {
		names = append(names, nick)
	}

	reply(client,
		fmt.Sprintf("%s :%s\r\n",
			channel.Name,
			strings.Join(names, " ")))

	reply(client,
		("End of /NAMES list\r\n"))
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
			reply(client, "PONG\r\n")
			return
		}
		msg := strings.Join(words[1:], " ")
		reply(client, fmt.Sprintf("PONG %s\r\n", msg))

	case "JOIN":
		if !client.registered {
			reply(client, "You have not registered\r\n")
			return
		}

		handleJoin(client, words)

	case "PRIVMSG":
		if !client.registered {
			reply(client, "You have not registered\r\n")
			return
		}

		handleMessage(client, words)
	}

	tryRegister(client)
}

func handleMessage(client *clientInfo, words []string) {
	if len(words) < 3 {
		reply(client, "PRIVMSG - Not enough parameters\r\n")
		return
	}

	channelName := words[1]

	if !strings.HasPrefix(channelName, "#") {
		reply(client, "Invalid channel name\r\n")
		return
	}

	channel, exists := serverState.channels[channelName]
	if !exists {
		reply(client, "Channel doesn't exist\r\n")
		return
	}

	msg := strings.Join(words[2:], " ")
	msg = strings.TrimPrefix(msg, ":")

	for _, channelClient := range channel.Members {
		if channelClient != client {
			reply(channelClient, fmt.Sprintf("%s: %s\r\n", client.NICK, msg))
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

	reply(client,
		"Welcome to IRC\r\n")
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
		reply(client, ":Nickname already in use\r\n")
		return
	}

	oldNick := client.NICK

	if oldNick != "" {
		delete(serverState.clients, oldNick)
	}

	client.NICK = newNick

	serverState.clients[newNick] = client
}

func handleUser(client *clientInfo, line string) {
	split := strings.SplitN(line, ":", 2)

	if len(split) < 2 {
		return
	}

	client.RealName = split[1]
}

func reply(client *clientInfo, msg string) {
	fmt.Println("send:", msg)

	_, err := client.Conn.Write([]byte(msg))
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
