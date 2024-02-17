package main

import (
	"fmt"
	"pgm"
)

func main() {
	client := pgm.CreateClientProtocol()
	go client.SendMessage([]byte("message"))

	server := pgm.CreateServerProtocol()
	go server.Listen()

	var input string
	fmt.Scan(&input)
}
