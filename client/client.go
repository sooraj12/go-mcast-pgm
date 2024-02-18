package main

import (
	"fmt"
	"pgm"
)

func main() {
	destIPS := []string{"192.168.1.3"}
	client := pgm.CreateClientProtocol()
	go client.SendMessage([]byte("message"), destIPS)

	var input string
	fmt.Scan(&input)
}
