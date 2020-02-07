package main

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrShutdown is the error returned when the server is shutting down
	ErrShutdown = errors.New("server is shutting down")
)

// Message is the broadcasted message
type Message struct {
	Data string
}

// Server is the vessel to send and receive messages
type Server struct {
	mu     sync.Mutex
	buffer chan *Message
	recvs  map[uint]func(*Message)
	next   uint
	done   <-chan struct{}
	wg     sync.WaitGroup
}

// NewServer creates a new server
func NewServer(bufferSize, capacity int) *Server {
	done := make(chan struct{})
	close(done)

	return &Server{
		buffer: make(chan *Message, bufferSize),
		recvs:  make(map[uint]func(*Message), capacity),
		done:   done,
	}
}

// Serve starts the server
func (s *Server) Serve(ctx context.Context) {
	s.done = ctx.Done()

	go func() {
		for {
			select {
			case m := <-s.buffer:
				s.mu.Lock()
				for _, f := range s.recvs {
					go f(m)
				}
				s.mu.Unlock()
			case <-s.done:
				return
			}
		}
	}()
}

func (s *Server) Subscribers() int {
	s.mu.Lock()
	count := len(s.recvs)
	s.mu.Unlock()
	return count
}

func (s *Server) Wait() {
	s.wg.Wait()
}

// Publish adds a message to the queue
func (s *Server) Publish(ctx context.Context, m *Message) error {
	select {
	case s.buffer <- m:
		return nil
	case <-ctx.Done():
		return nil
	case <-s.done:
		return ErrShutdown
	}
}

// Subscribe waits for messages to be received
func (s *Server) Subscribe(ctx context.Context, f func(*Message)) error {
	s.wg.Add(1)
	defer s.wg.Done()

	s.mu.Lock()
	idx := s.next
	s.recvs[idx] = f
	s.next++ // this could potentially overflow and cause unexpected behavior
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.recvs, idx)
		s.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return nil
	case <-s.done:
		return ErrShutdown
	}
}
