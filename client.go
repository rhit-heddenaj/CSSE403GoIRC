package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
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

	nickCommand := "NICK " + username + "\r\n"
	nickByte := []byte(nickCommand)

	conn.Write(nickByte)

	userCommand := "USER " + username + " * * :Hunter Two\r\n"
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
