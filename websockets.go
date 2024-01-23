package sockjsclient

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/dchest/uniuri"
	"github.com/gorilla/websocket"
)

type WebSocket struct {
	sync.Mutex
	Address          string
	TransportAddress string
	ServerID         string
	SessionID        string
	Connection       *websocket.Conn
	Inbound          chan []byte
	Reconnected      chan struct{}
	RequestHeaders   http.Header
	Jar              http.CookieJar
}

func NewWebSocket(address string, headers http.Header, jar http.CookieJar) (*WebSocket, error) {
	ws := &WebSocket{
		Address:        address,
		ServerID:       paddedRandomIntn(999),
		SessionID:      uniuri.New(),
		Inbound:        make(chan []byte),
		Reconnected:    make(chan struct{}, 32),
		RequestHeaders: headers,
		Jar:            jar,
	}

	ws.TransportAddress = address + "/" + ws.ServerID + "/" + ws.SessionID + "/websocket"

	ws.Loop()

	return ws, nil
}

func (w *WebSocket) Loop() {
	dialer := *websocket.DefaultDialer
	dialer.Jar = w.Jar

	go func() {
		err := backoff.Retry(func() error {
			log.Printf("Starting a WebSocket connection to %s", w.TransportAddress)

			ws, _, err := dialer.Dial(w.TransportAddress, w.RequestHeaders)
			if err != nil {
				return err
			}

			// Read the open message
			_, data, err := ws.ReadMessage()
			if err != nil {
				return err
			}

			if data[0] != 'o' {
				return errors.New("invalid initial message")
			}

			w.Connection = ws

			w.Reconnected <- struct{}{}

			for {
				_, data, err := w.Connection.ReadMessage()
				if err != nil {
					return err
				}

				if len(data) < 1 {
					continue
				}

				switch data[0] {
				case 'h':
					// Heartbeat
					continue
				case 'a':
					// Normal message
					w.Inbound <- data[1:]
				case 'c':
					// Session closed
					var v []interface{}
					if err := json.Unmarshal(data[1:], &v); err != nil {
						log.Printf("Closing session: %s", err)
						return nil
					}
					return errors.New("connection closed")
				}
			}
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.Print(err)
		}
	}()

	<-w.Reconnected
}

func (w *WebSocket) Read(v []byte) (int, error) {
	data := <-w.Inbound
	n := copy(v, data)
	return n, nil
}

func (w *WebSocket) Write(v []byte) (int, error) {
	return len(v), w.Connection.WriteMessage(1, v)
}

func (w *WebSocket) Close() error {
	return w.Connection.Close()
}
