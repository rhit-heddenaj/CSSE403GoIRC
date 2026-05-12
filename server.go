package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

const RplWelcome = 1

func handleConnection(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Fields(line)

		fmt.Println("recv: " + line)

		reply := ""
		switch words[0] {
		case "PING":
			reply = fmt.Sprintf("PONG %s\r\n", words[1])
		case "USER":
			reply = fmt.Sprintf("%03d %s :hi\r\n", RplWelcome, words[1])
		}

		if reply != "" {
			fmt.Println("send: " + reply)
			_, err := conn.Write([]byte(reply))
			if err != nil {
				fmt.Println("Error sending reply to conn: ", err)
			}
		}
	}
}

func main() {
	ln, err := net.Listen("tcp", ":6667")
	if err != nil {
		fmt.Println("Error creating Listener: ", err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting conn: ", err)
		}
		go handleConnection(conn)
	}
}
