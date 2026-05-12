package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func main() {
	connect_port("irc.libera.chat", "6667")
}

func connect_port(host string, port string) {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		fmt.Println("Connecting Error: ", err)
	}
	if conn != nil {
		defer conn.Close()
		fmt.Println("Opened", net.JoinHostPort(host, port))
		status, err := bufio.NewReader(conn).ReadString('\n')
		fmt.Println(status)
		fmt.Println(err)
		connect_user(conn)
	}
}

func connect_user(conn net.Conn) {

	nickCommand := "NICK testUser\r\n"
	nickByte := []byte(nickCommand)

	conn.Write(nickByte)

	userCommand := "USER testUser * * :Hunter Two\r\n"
	userByte := []byte(userCommand)

	conn.Write(userByte)

	joinCommand := "JOIN #testServerChannel\r\n"
	conn.Write([]byte(joinCommand))

	for {
		status, err := bufio.NewReader(conn).ReadString('\n')
		fmt.Println(status)
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}
