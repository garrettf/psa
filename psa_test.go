package psa

import (
	"fmt"
	"sync"
	"testing"
)

func benchMessages(numMessages int, numSubs int, b *testing.B) {
	pub := NewPublisher(9853)
	pub.Listen()

	wg := sync.WaitGroup{}
	wg.Add(numSubs)

	for i := 0; i < numSubs; i++ {
		s := NewSubscriber()
		subChan, err := s.Subscribe("localhost:9853")
		if err != nil {
			b.Fatal("Error while subscribing:", err)
		}

		go func(subChan *chan Message, wg *sync.WaitGroup, sub *Subscriber) {
			wg.Done()
			j := 0
			for _ = range *subChan {
				j += 1
				if j >= numMessages {
					break
				}
			}
			wg.Done()
			sub.Stop()
		}(&subChan, &wg, s)
	}

	wg.Wait()
	wg.Add(numSubs + 1)
	b.ResetTimer()
	// Publish messages
	go func() {
		for i := 0; i < numMessages; i++ {
			pub.Publish(Message{Data: StringData{Str: fmt.Sprintln(i)}})
		}
		wg.Done()
	}()

	// Wait for all messages to be published and all subscribers to be done reading
	wg.Wait()
	b.StopTimer()
	pub.Close()
}

func Benchmark1000MsgsTo500Subs(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchMessages(1000, 500, b)
	}
}
