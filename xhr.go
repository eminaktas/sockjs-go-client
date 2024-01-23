package sockjsclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"sync"

	"github.com/dchest/uniuri"
	"github.com/igm/sockjs-go/v3/sockjs"
)

type XHR struct {
	Address          string
	TransportAddress string
	ServerID         string
	SessionID        string
	Inbound          chan []byte
	Done             chan bool
	sessionState     sockjs.SessionState
	mu               sync.RWMutex
}

var client = http.Client{Timeout: time.Second * 10}

func NewXHR(address string) (*XHR, error) {
	xhr := &XHR{
		Address:      address,
		ServerID:     paddedRandomIntn(999),
		SessionID:    uniuri.New(),
		Inbound:      make(chan []byte),
		Done:         make(chan bool, 1),
		sessionState: sockjs.SessionOpening,
	}
	xhr.TransportAddress = address + "/" + xhr.ServerID + "/" + xhr.SessionID
	if err := xhr.Init(); err != nil {
		return nil, err
	}
	go xhr.StartReading()

	return xhr, nil
}

func (x *XHR) Init() error {
	req, err := http.NewRequest("POST", x.TransportAddress+"/xhr", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if body[0] != 'o' {
		return errors.New("invalid initial message")
	}
	x.setSessionState(sockjs.SessionActive)

	return nil
}

// func (x *XHR) doneNotify() {
// 	return
// }

func (x *XHR) StartReading() {
	client := &http.Client{Timeout: time.Second * 30}

	for {
		select {
		case <-x.Done:
			return
		default:
			req, err := http.NewRequest("POST", x.TransportAddress+"/xhr", nil)
			if err != nil {
				log.Print(err)
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				log.Print(err)
				continue
			}

			data, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				log.Print(err)
				continue
			}

			switch data[0] {
			case 'h':
				// Heartbeat
				continue
			case 'a':
				// Normal message
				x.Inbound <- data[1:]
			case 'c':
				// Session closed
				x.setSessionState(sockjs.SessionClosed)
				var v []interface{}
				if err := json.Unmarshal(data[1:], &v); err != nil {
					log.Printf("closing session: %s", err)
					return
				}
				log.Printf("%v: %v", v[0], v[1])
				return
			}
		}
	}
}

func (x *XHR) Read(v []byte) (int, error) {
	data := <-x.Inbound
	n := copy(v, data)
	return n, nil
}

func (x *XHR) Write(v []byte) (int, error) {
	req, err := http.NewRequest("POST", x.TransportAddress+"/xhr_send", bytes.NewReader(v))
	if err != nil {
		return len(v), err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return len(v), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return len(v), errors.New("invalid HTTP code - " + resp.Status)
	}

	return len(v), nil
}

func (x *XHR) Close() error {
	select {
	case x.Done <- true:
	default:
		return fmt.Errorf("error closing XHR")
	}
	return nil
}

func (x *XHR) GetSessionState() sockjs.SessionState {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.sessionState
}

func (x *XHR) setSessionState(state sockjs.SessionState) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.sessionState = state
}
