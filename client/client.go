package main

import (
	"fmt"
	"pgm"
)


func main() {
	client := pgm.CreateClientProtocol()
	go client.SendMessage([]byte("message"))

	var input string
	fmt.Scan(&input)
}
