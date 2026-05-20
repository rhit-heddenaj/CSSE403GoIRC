package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

func client() {

	host := "irc.libera.chat"
	port := "6667"
	var username, realName string

	args := os.Args[2:]

	if len(args) < 1 {
		fmt.Println("Usage: go run . client <nick> [host] [port] [realName]")
		fmt.Println("Example: go run . client mynick irc.libera.chat 6667 \"My Real Name\"")
		return
	}

	username = args[0]

	if len(args) > 1 {
		host = args[1]
	}
	if len(args) > 2 {
		port = args[2]
	}
	if len(args) > 3 {
		realName = args[3]
	} else {
		realName = "Go IRC Client"
	}

	conn := connectPort(host, port)

	if conn == nil {
		return
	}

	defer conn.Close()

	connectUser(username, realName, conn)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:                 "> ",
		DisableAutoSaveHistory: true,
	})
	if err != nil {
		fmt.Println("failed to initialize terminal input:", err)
		return
	}
	defer rl.Close()

	go listenServer(conn, rl)

	repl(conn, rl)
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func listenServer(conn net.Conn, rl *readline.Instance) {
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Server disconnected:", err)
			return
		}

		if _, err := rl.Write([]byte(message)); err != nil {
			fmt.Println("failed to write server output:", err)
			return
		}
	}
}

func repl(conn net.Conn, rl *readline.Instance) {
	for {
		input, err := rl.Readline()
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Println("readline error:", err)
			break
		}
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
			conn.Write([]byte("QUIT\r\n"))
			return

		case "PART":
			if len(words) < 2 {
				fmt.Println("Usage: PART #channel")
				continue
			}
			conn.Write([]byte("PART " + words[1] + "\r\n"))

		case "LIST":
			conn.Write([]byte("LIST\r\n"))

		case "NAMES":
			if len(words) < 2 {
				fmt.Println("Usage: NAMES #channel")
				continue
			}
			conn.Write([]byte("NAMES " + words[1] + "\r\n"))

		case "TOPIC":
			if len(words) < 2 {
				fmt.Println("Usage: TOPIC #channel topic || Usage: TOPIC #channel")
				continue
			}
			msg := strings.Join(words[1:], " ")
			conn.Write([]byte("TOPIC " + msg + "\r\n"))

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

func connectUser(username string, realName string, conn net.Conn) {
	conn.Write([]byte("NICK " + username + "\r\n"))
	conn.Write([]byte("USER " + username + " * * :" + realName + "\r\n"))
}
