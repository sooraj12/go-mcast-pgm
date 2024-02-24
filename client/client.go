package main

import (
	"pgm"
	"sync"
)

func main() {
	wait := &sync.WaitGroup{}
	destIPS := []string{"192.168.1.2"}
	client := pgm.CreateClientProtocol()
	wait.Add(1)
	go client.SendMessage([]byte("message"), destIPS)
	wait.Wait()
}
