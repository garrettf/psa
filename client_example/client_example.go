package main

import (
	"github.com/garrettf/psa"
	"log"
	"os"
	"os/signal"
)

var terminate chan os.Signal

func main() {
	s := psa.NewSubscriber()
	subChan, err := s.Subscribe("localhost:9853")
	if err != nil {
		log.Println("failed to subscribe to publisher:", err)
	}
	for msg := range subChan {
		log.Println("<-", msg)
	}

	/* Handle ctrl+C */
	terminate = make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	s.Stop()
}
