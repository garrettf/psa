package main

import (
	"fmt"
	"github.com/garrettf/psa"
	"log"
	"os"
	"os/signal"
	"time"
)

var terminate chan os.Signal

func main() {
	pub := psa.NewPublisher(9853)
	err := pub.Listen()
	if err != nil {
		log.Println("Failed to start listen server: ", err)
		return
	}
	log.Println("Listening on :9853")

	go func(pub *psa.Publisher) {
		i := 1
		for {
			msg := psa.Message{Data: psa.StringData{Str: fmt.Sprintln("HELLO #", i)}}
			time.Sleep(500 * time.Millisecond)
			pub.Publish(msg)
			log.Println("->", msg)
			i += 1
		}
	}(pub)

	/* Handle ctrl+C */
	terminate = make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	pub.Close()
}
