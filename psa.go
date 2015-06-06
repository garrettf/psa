package psa

import (
	"encoding/gob"
	"github.com/garrettf/slll"
	"net"
	"strconv"
	"sync"
	"time"
)

type Message struct {
	Id   int
	Data interface{}
}

type SubscribeData struct {
	LastMsgId int
}

type StringData struct {
	Str string
}

type Publisher struct {
	port         uint16
	listener     *net.TCPListener
	notifier     *sync.Cond
	log          *slll.List
	lastSeenElem *slll.Element
	conWaiters   sync.WaitGroup
	closer       chan bool
}

func NewPublisher(port uint16) *Publisher {
	log := slll.NewList()
	return &Publisher{
		port:         port,
		notifier:     sync.NewCond(&sync.Mutex{}),
		log:          log,
		lastSeenElem: log.Head(),
		conWaiters:   sync.WaitGroup{},
		closer:       make(chan bool),
	}
}

func (p *Publisher) Listen() error {
	port := strconv.FormatInt(int64(p.port), 10)
	laddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:"+port)
	if err != nil {
		return err
	}
	p.listener, err = net.ListenTCP("tcp", laddr)
	if err != nil {
		return err
	}
	p.conWaiters.Add(1)
	go p.handleNewConnections()
	return nil
}

func (p *Publisher) Close() {
	close(p.closer)
	p.notifier.Broadcast()
	p.conWaiters.Wait()
}

func (p *Publisher) isClosing() bool {
	select {
	case <-p.closer:
		/* Respond to close signal */
		return true
	default:
		return false
	}
}

/* Accepts subscriber connections */
func (p *Publisher) handleNewConnections() {
	defer p.conWaiters.Done()
	defer p.listener.Close()
	for {
		if p.isClosing() {
			return
		}
		p.listener.SetDeadline(calcTimeout())
		conn, err := p.listener.Accept()
		if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
			// Passed deadline, try again
			continue
		}
		if err != nil {
			continue
		}
		// Successful connection
		p.conWaiters.Add(1)
		go p.handleSubscriber(conn)
	}
}

func (p *Publisher) handleSubscriber(conn net.Conn) {
	defer p.conWaiters.Done()
	defer conn.Close()

	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	/* Receive subscribe command */
	var msg Message

	for {
		if p.isClosing() {
			return
		}

		conn.SetDeadline(calcTimeout())
		err := dec.Decode(&msg)
		if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
			// Passed deadline, try again
			continue
		}
		if err != nil {
			return
		}
		break
	}

	switch msg.Data.(type) {
	case SubscribeData:
		// Send them all the messages
		lastElem := p.sendMessagesAfter(p.log.Head(), conn, enc)
		if lastElem == nil {
			// An error occurred.
			return
		}

		// Wait for next publish
		for {
			p.notifier.L.Lock()
			p.notifier.Wait()
			p.notifier.L.Unlock()
			lastElem = p.sendMessagesAfter(lastElem, conn, enc)
			if lastElem == nil {
				// An error occurred.
				return
			}
		}
	default:
		// Unrecognized message
	}
}

func (p *Publisher) sendMessagesAfter(e *slll.Element, conn net.Conn, enc *gob.Encoder) *slll.Element {
	if p.isClosing() {
		return nil
	}
	lastElem := e
	for msgElem := e.Next(); msgElem != nil; msgElem = msgElem.Next() {
		for {
			if p.isClosing() {
				return nil
			}

			conn.SetDeadline(calcTimeout())
			err := enc.Encode(msgElem.Value)
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				// Passed deadline, try again
				continue
			}
			if err != nil {
				return nil
			}
			break
		}
		lastElem = msgElem
	}
	return lastElem
}

func (p *Publisher) Publish(m Message) {
	p.lastSeenElem = p.lastSeenElem.PushBack(&m)
	p.notifier.Broadcast()
}

type Subscriber struct {
	conn      net.Conn
	enc       *gob.Encoder
	dec       *gob.Decoder
	messages  chan Message
	stopper   chan bool
	conWaiter sync.WaitGroup
}

func NewSubscriber() *Subscriber {
	return &Subscriber{
		messages:  make(chan Message, 128),
		stopper:   make(chan bool),
		conWaiter: sync.WaitGroup{},
	}
}

func (s *Subscriber) isClosing() bool {
	select {
	case <-s.stopper:
		/* Respond to close signal */
		return true
	default:
		return false
	}
}

func (s *Subscriber) Subscribe(host string) (chan Message, error) {
	var err error

	s.conn, err = net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	s.enc = gob.NewEncoder(s.conn)
	s.dec = gob.NewDecoder(s.conn)

	// Send subscribe command
	err = s.enc.Encode(Message{Data: SubscribeData{}})
	if err != nil {
		s.conn.Close()
		return nil, err
	}

	s.conWaiter.Add(1)
	go s.handleConnection()
	return s.messages, nil
}

func (s *Subscriber) Stop() {
	close(s.stopper)
	s.conWaiter.Wait()
}

func (s *Subscriber) handleConnection() {
	defer s.conWaiter.Done()
	defer close(s.messages)
	defer s.conn.Close()

	for {
		if s.isClosing() {
			return
		}

		/* Receive a message */
		var msg Message
		s.conn.SetDeadline(calcTimeout())
		err := s.dec.Decode(&msg)

		if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
			// Passed deadline, try again
			continue
		}
		if err != nil {
			return
		}

		s.messages <- msg
	}
}

func calcTimeout() time.Time {
	return time.Now().Add(500 * time.Millisecond)
}

func init() {
	gob.Register(SubscribeData{})
	gob.Register(StringData{})
}
