package main_test

import (
	"context"

	. "github.com/smousa/coder-pubsub"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {

	var (
		ctx    context.Context
		cancel context.CancelFunc
		s      *Server
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		s = NewServer(0, 1)
	})

	AfterEach(func() {
		cancel()
	})

	Context("with a stopped server", func() {
		It("should not publish messages", func() {
			err := s.Publish(ctx, &Message{Data: "hello world!"})
			Ω(err).Should(Equal(ErrShutdown))
		})

		It("should not receive messages", func() {
			err := s.Subscribe(ctx, func(m *Message) {
				Fail("receive unexpected message")
			})
			Ω(err).Should(Equal(ErrShutdown))
		})
	})

	Context("with a running server", func() {
		BeforeEach(func() {
			s.Serve(ctx)
		})

		AfterEach(func() {
			s.Wait()
		})

		It("should publish a message", func() {
			msg := &Message{Data: "hello world!"}

			err := s.Publish(ctx, msg)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("should subscribe to messages and return an error on shutdown", func() {
			expected := []*Message{
				{
					Data: "aaaa",
				}, {
					Data: "bbbb",
				}, {
					Data: "cccc",
				},
			}

			var ch = make(chan *Message)
			go func() {
				defer close(ch)
				err := s.Subscribe(context.Background(), func(m *Message) {
					ch <- m
				})
				Ω(err).Should(Equal(ErrShutdown))
			}()

			Eventually(s.Subscribers).Should(Equal(1))

			for _, m := range expected {
				m := m
				err := s.Publish(context.Background(), m)
				Ω(err).ShouldNot(HaveOccurred())
			}

			var actual []*Message
			for m := range ch {
				actual = append(actual, m)
				if len(actual) == len(expected) {
					cancel()
				}
			}
			Ω(actual).Should(ConsistOf(expected))
		})

		It("should subscribe to messages and return nil on cancel", func() {
			expected := []*Message{
				{
					Data: "aaaa",
				}, {
					Data: "bbbb",
				}, {
					Data: "cccc",
				},
			}

			subctx, subcancel := context.WithCancel(context.Background())

			var ch = make(chan *Message)
			go func() {
				defer close(ch)
				err := s.Subscribe(subctx, func(m *Message) {
					ch <- m
				})
				Ω(err).ShouldNot(HaveOccurred())
			}()

			Eventually(s.Subscribers).Should(Equal(1))

			for _, m := range expected {
				m := m
				err := s.Publish(context.Background(), m)
				Ω(err).ShouldNot(HaveOccurred())
			}

			var actual []*Message
			for m := range ch {
				actual = append(actual, m)
				if len(actual) == len(expected) {
					subcancel()
				}
			}
			Ω(actual).Should(ConsistOf(expected))
		})
	})
})
