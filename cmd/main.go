package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bxra2/GO-HTMX-webapp/internal/hardware"
	"nhooyr.io/websocket"
)

type server struct {
	subscriberMessageBuffer int
	mux                     http.ServeMux
	subscribersMutex        sync.Mutex
	subscribers             map[*subscriber]struct{}
}

type subscriber struct {
	msgs chan []byte
}

func NewServer() *server {
	s := &server{
		subscriberMessageBuffer: 10,
		subscribers:             make(map[*subscriber]struct{})}

	s.mux.Handle("/", http.FileServer(http.Dir("./htmx")))
	s.mux.HandleFunc("/ws", s.subscribeHandler)
	return s
}

func (s *server) subscribeHandler(w http.ResponseWriter, r *http.Request) {
	err := s.subscribe(r.Context(), w, r)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func (s *server) subscribe(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var c *websocket.Conn
	subscriber := &subscriber{
		msgs: make(chan []byte, s.subscriberMessageBuffer),
	}
	s.addSubscriber(subscriber)

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return err
	}
	defer c.CloseNow()

	ctx = c.CloseRead(ctx)
	for {
		select {
		case msg := <-subscriber.msgs:
			ctx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			err := c.Write(ctx, websocket.MessageText, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *server) addSubscriber(subscriber *subscriber) {
	s.subscribersMutex.Lock()
	s.subscribers[subscriber] = struct{}{}
	s.subscribersMutex.Unlock()
	fmt.Println("added subscriber", subscriber)
}

func (s *server) broadcast(msg []byte) {
	s.subscribersMutex.Lock()
	for subscriber := range s.subscribers {
		subscriber.msgs <- msg
	}
	s.subscribersMutex.Unlock()

}

func main() {
	fmt.Println("Starting System Mointor...")
	srv := NewServer()
	go func(s *server) {
		for {
			systemSection, err := hardware.GetSystemSection()
			if err != nil {
				fmt.Println(err)
			}

			diskSection, err := hardware.GetDiskSection()
			if err != nil {
				fmt.Println(err)
			}

			cpuSection, err := hardware.GetCpuSection()
			if err != nil {
				fmt.Println(err)
			}

			timeStamp := time.Now().Format("2006-01-02 15:04:05")

			html := `
			<div hx-swap-oob="innerHTML:#update-timestamp">
			` + timeStamp + `
			</div>
			<h3>system</h3>
			<br/>
			<div hx-swap-oob="innerHTML:#system-data">
			` + systemSection + `
			</div>
			<h3>disk</h3>
			<br/>

			<div hx-swap-oob="innerHTML:#disk-data">
			` + diskSection + `
			</div>
			<h3>cpu</h3>
			<br/>

			<div hx-swap-oob="innerHTML:#cpu-data">
			` + cpuSection + `
			</div>

			`

			s.broadcast([]byte(html))

			time.Sleep(3 * time.Second)

		}
	}(srv)

	err := http.ListenAndServe(":8080", &srv.mux)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
