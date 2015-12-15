package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

type HttpAuth struct {
	b      *batcher
	port   string // port to listen on
	socket net.Listener
}

// Start starts the httpAuth servers listening on the requested port
// and sends connection details to the batcher
func startHttp(port string, b *batcher) (*HttpAuth, error) {

	h := &HttpAuth{
		port: port,
		b:    b,
	}

	// start listening on a background goroutine
	go h.listenForConn()

	return h, nil
}

// Close will close the listening server and shutdown the system
func (h *HttpAuth) Close() {
	h.socket.Close()
}

// listenForConn runs in a goroutine to listen for incoming connections
func (h *HttpAuth) listenForConn() {

	http.HandleFunc("/", authHandler)
	//
	err := http.ListenAndServe(":"+h.port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// authHandler is called in a goroutine to handle each incoming request
func authHandler(w http.ResponseWriter, r *http.Request) {

	// pull the auth details from the request
	user, pass, ok := r.BasicAuth()
	if ok == true {

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = ""
		}
		r := &AuthEvent{
			Time:        fmt.Sprintf("%d", time.Now().Unix()),
			AuthType:    "httpAuth",
			SrcIP:       host,
			DestIP:      extIP,
			User:        user,
			Credentials: strconv.QuoteToASCII(pass),
		}
		addToBatch(r)

	}
	// always return auth fail
	w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
	http.Error(w, "authorization failed", http.StatusUnauthorized)
}

/*

 */
