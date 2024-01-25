package sockjsclient

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/dchest/uniuri"
	"github.com/gorilla/websocket"
)

// WebSocket represents a SockJS WebSocket connection.
type WebSocket struct {
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

// NewWebSocket creates a new WebSocket instance.
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
				return backoff.Permanent(errors.New("invalid initial message"))
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
					close(w.Inbound)
					return backoff.Permanent(errors.New("connection closed"))
				}
			}
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.Print(err)
		}
	}()

	<-w.Reconnected
}

// Read reads a message from the WebSocket and unquotes it.
func (w *WebSocket) Read(v []byte) (int, error) {
	// Wait for an inbound message from the channel.
	message := <-w.Inbound

	// Convert the received message to a string for processing.
	messageString := string(message)

	// Trim the brackets "[" and "]" from the string.
	messageString = strings.TrimPrefix(messageString, "[")
	messageString = strings.TrimSuffix(messageString, "]")

	// Unquote the string to handle escaped characters.
	unquoted, err := strconv.Unquote(messageString)
	if err != nil {
		return 0, err
	}

	// Copy the unquoted message to the provided byte slice.
	n := copy(v, []byte(unquoted))

	// Return the number of bytes copied and a nil error.
	return n, nil
}

// Write writes a message to the WebSocket after formatting and escaping it.
func (w *WebSocket) Write(v []byte) (int, error) {
	// Format the byte slice as a Go-syntax quoted string.
	quotedMessage := fmt.Sprintf("[%q]", v)

	// Replace occurrences of null character (\x00) with Unicode escape sequence (\u0000).
	// This is done to ensure compatibility with systems that may not recognize \x00.
	escapedMessage := strings.ReplaceAll(quotedMessage, `\x00`, `\u0000`)

	// Convert the escaped message back to a byte slice for transmission.
	message := []byte(escapedMessage)

	// Write the message to the WebSocket connection.
	return len(message), w.Connection.WriteMessage(websocket.TextMessage, message)
}

// Close closes the WebSocket connection.
func (w *WebSocket) Close() error {
	return w.Connection.Close()
}
