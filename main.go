package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	log := logrus.WithContext(ctx)

	s := NewServer(0, 100)
	s.Serve(ctx)

	// set up routing
	r := mux.NewRouter()

	r.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		// decode the message
		var m Message
		err := json.NewDecoder(r.Body).Decode(&m)
		if err != nil {
			http.Error(w, "invalid request message", http.StatusBadRequest)
			return
		}

		// publish the message
		err = s.Publish(r.Context(), &m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}).Methods("POST")

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	r.HandleFunc("/subscribe", func(w http.ResponseWriter, r *http.Request) {
		// upgrade the connection
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.WithError(err).Error("Could not upgrade to websocket connection")
			return
		}

		ctx, cancel := context.WithCancel(r.Context())

		// subscribe to messages
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			var mu sync.Mutex
			err := s.Subscribe(ctx, func(m *Message) {
				mu.Lock()
				if err := conn.WriteJSON(m); err != nil {
					log.WithError(err).Error("Could not write to websocket connection")
					cancel()
				}
				mu.Unlock()
			})
			if err != nil {
				log.WithError(err).Error("Could not receive message")
			}
		}()

		for {
			_, _, err := conn.NextReader()
			if err != nil {
				// cancel subscription and clean up mess
				cancel()
				wg.Wait()
				return
			}
		}
	})

	// set up and start the server
	srv := &http.Server{
		Addr:         ":8080",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server exited with error")
			return
		}
		log.Info("Stopped accepting http requests")
	}()

	log.WithField("addr", srv.Addr).Info("Server started")

	// wait for signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	srv.Shutdown(ctx)
	wg.Wait()

	cancel()
	s.Wait()
	log.Info("Server stopped")
}
