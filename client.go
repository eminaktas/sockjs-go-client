package sockjsclient

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Client struct {
	Connection Connection

	WebSockets   bool
	Address      string
	ReadBufSize  int
	WriteBufSize int

	Reconnected    chan struct{}
	RequestHeaders http.Header
	Jar            http.CookieJar
}

func NewClient(address string, headers http.Header, jar http.CookieJar) (*Client, error) {
	client := &Client{RequestHeaders: headers, Jar: jar}

	client.Address = address

	// Get info whether WebSockets are enabled
	info, err := client.Info()
	if err != nil {
		return nil, err
	}
	client.WebSockets = info.WebSocket

	// Create a WS session (not a SJS one)
	if client.WebSockets {
		a2 := strings.Replace(address, "https", "wss", 1)
		a2 = strings.Replace(a2, "http", "ws", 1)

		ws, err := NewWebSocket(a2, headers, jar)
		if err != nil {
			return nil, err
		}

		client.Connection = ws
		client.Reconnected = ws.Reconnected
	} else {
		// XHR
		client.Connection, err = NewXHR(address)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (c *Client) Info() (*Info, error) {
	client := http.Client{Jar: c.Jar}
	req, err := http.NewRequest(http.MethodGet, c.Address+"/info", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	var info *Info
	if err := dec.Decode(&info); err != nil {
		return nil, err
	}

	return info, nil
}

func (c *Client) Write(p []byte) (int, error) {
	return c.Connection.Write(p)
}

func (c *Client) Read(p []byte) (int, error) {
	return c.Connection.Read(p)
}

func (c *Client) Close() error {
	return c.Connection.Close()
}

func (c *Client) ForceClose() {
	c.Connection.ForceClose()
}
