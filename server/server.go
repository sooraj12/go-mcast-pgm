package main

import (
	"pgm"
	"sync"
)

func main() {
	wait := &sync.WaitGroup{}
	server := pgm.CreateServerProtocol()
	wait.Add(1)
	go server.Listen()
	wait.Wait()
}
