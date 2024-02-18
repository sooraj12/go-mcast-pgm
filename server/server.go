package main

import (
	"fmt"
	"pgm"
)


func main() {
	server := pgm.CreateServerProtocol()
	go server.Listen()

	var input string
	fmt.Scan(&input)
}
