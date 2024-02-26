package main

import (
	"pgm"
	"strings"
	"sync"
)

func main() {
	wait := &sync.WaitGroup{}
	destIPS := []string{"192.168.1.8"}
	client := pgm.CreateClientProtocol()
	wait.Add(1)
	str := strings.Repeat("m", 1700)
	go client.SendMessage([]byte(str), destIPS)
	wait.Wait()
}
