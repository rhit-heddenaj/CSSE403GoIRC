package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  go run . server")
		fmt.Println("  go run . client <nick> [host] [port] [realName]")
		fmt.Println("Examples:")
		fmt.Println("  go run . client mynick")
		fmt.Println("  go run . client mynick irc.libera.chat 6667 \"My Real Name\"")
		return
	}

	switch os.Args[1] {
	case "server":
		runServer("6667")
	case "server2":
		runServerOnNetwork("6668", "localhost", "6667")
	case "server3":
		runServerOnNetwork("6669", "localhost", "6667")
	case "client":
		client()
	default:
		fmt.Println("Unknown mode:", os.Args[1])
	}
}
