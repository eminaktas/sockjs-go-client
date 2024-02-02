# sockjs-go-client

**Optimized for STOMP Protocol...**

## Overview

`sockjs-go-client` is a client library optimized for use with the STOMP (Simple Text Oriented Messaging Protocol) over SockJS in Go applications. It provides a convenient and efficient way to establish WebSocket and XHR connections with a SockJS server, specifically tailored for use with the STOMP protocol.

## Installation

```bash
go get -u github.com/eminaktas/sockjs-go-client
```

## Usage

Here's a quick example demonstrating how to create and use a SockJS connection with STOMP protocol.

For the server, you can use the `sockjs-stomp-go-server` project. Refer to the [example provided here](https://github.com/eminaktas/sockjs-stomp-go-server/blob/main/example/main.go) for a server-side implementation.

```go
package main

import (
 "log"
 "net/http"
 "time"

 sockjsclient "github.com/eminaktas/sockjs-go-client"
 "github.com/go-stomp/stomp/v3"
)

func main() {
 // Define SockJS server address
 serverAddress := "http://localhost:8085/connect"

 // Create a new SockJS client instance
 headers := make(http.Header)
 jar := new(http.CookieJar)
 sockJSClient, err := sockjsclient.NewClient(serverAddress, headers, *jar)
 if err != nil {
  log.Println("Error creating SockJS client:", err)
  return
 }

 log.Println("SockJS connection successful")

 // Define STOMP connection options
 stompHost := "default"
 stompSendTimeout := 1000 * time.Millisecond
 stompRecvTimeout := 1000 * time.Millisecond

 var stompOptions []func(*stomp.Conn) error = []func(*stomp.Conn) error{
  stomp.ConnOpt.Host(stompHost),                               // Set the host for the STOMP connection
  stomp.ConnOpt.HeartBeat(stompSendTimeout, stompRecvTimeout), // Configure heartbeats
 }

 // Connect to the STOMP server using SockJS as the underlying transport
 stompConnection, err := stomp.Connect(sockJSClient, stompOptions...)
 if err != nil {
  log.Println("Error creating STOMP connection:", err.Error())
  return
 }
 defer func() {
  log.Println("Disconnecting from STOMP server...")
  stompConnection.Disconnect()
 }()

 log.Println("STOMP connection successful")

 // Send a message to the specified destination
 sendDestination := "/echo/"
 messageBody := "an example message"
 log.Printf("Sending message to '%s': %s\n", sendDestination, messageBody)
 _ = stompConnection.Send(sendDestination, "text/plain", []byte(messageBody))

 // Subscribe to a destination on the STOMP server
 subscribeDestination := "/topic"
 subscription, err := stompConnection.Subscribe(subscribeDestination, stomp.AckAuto)
 if err != nil {
  log.Printf("Subscription failed: %v\n", err)
  return
 }

 // Wait for the response from the subscribed destination
 receivedMessage := <-subscription.C
 if receivedMessage.Err != nil {
  log.Printf("message recieve failed: %v\n", receivedMessage.Err)
  return
 }
 log.Printf("Received Message from '%s': %s\n", subscribeDestination, receivedMessage.Body)

 // Unsubscribe after message receieved.
 subscription.Unsubscribe()
}
```
