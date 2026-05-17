package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func client() {
	conn := connectPort("127.0.0.1", "6667")

	if conn == nil {
		return
	}

	defer conn.Close()

	user := os.Args[2]
	connectUser(user, conn)

	go listenServer(conn)

	repl(conn)
}

func listenServer(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Server disconnected:", err)
			return
		}
		fmt.Print("\r" + message)
		fmt.Print("> ")
	}
}

func repl(conn net.Conn) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		if input == "" {
			continue
		}

		words := strings.Fields(input)
		switch words[0] {
		case "PING":
			if len(words) == 1 {
				conn.Write([]byte("PING\r\n"))
				continue
			}

			msg := strings.Join(words[1:], " ")
			conn.Write([]byte("PING " + msg + "\r\n"))
		case "JOIN":
			if len(words) < 2 {
				fmt.Println("Usage: JOIN #channel")
				continue
			}
			conn.Write([]byte("JOIN " + words[1] + "\r\n"))

		case "MSG":
			if len(words) < 3 {
				fmt.Println("Usage: MSG #channel <message>")
				continue
			}
			msg := strings.Join(words[2:], " ")
			conn.Write([]byte("PRIVMSG " + words[1] + " :" + msg + "\r\n"))

		case "QUIT":
			return

		default:
			fmt.Println("Unknown command:", words[0])
		}
	}
}

func connectPort(host string, port string) net.Conn {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		fmt.Println("Connecting Error: ", err)
		return nil
	}
	if conn != nil {

		fmt.Println("Opened", net.JoinHostPort(host, port))
	}

	return conn
}

func connectUser(username string, conn net.Conn) {
	conn.Write([]byte("NICK " + username + "\r\n"))
	conn.Write([]byte("USER " + username + " * * :Hunter Two\r\n"))
	conn.Write([]byte("JOIN #testServerChannel\r\n"))
}
