package httpauth

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gombadi/honeygot/code/honeygot/batcher"
)

type HttpAuth struct {
	port   string // port to listen on
	socket net.Listener
}

func New() *HttpAuth {
	return &HttpAuth{port: "48484"}
}

// Start starts the httpAuth servers listening on the requested port
// and sends connection details to the batcher
func (h *HttpAuth) Start() error {

	if port := os.Getenv("HONEYGOT_HTTPPORT"); port != "" {
		h.port = port
	}

	// start listening on a background goroutine
	go h.listenForConn()

	return nil
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

		batcher.AddToBatch(fmt.Sprintf("%d httpAuth %s %s",
			time.Now().Unix(),
			strings.Split(r.RemoteAddr, ":")[0],
			base64.StdEncoding.EncodeToString([]byte(user+":"+pass))))
	}
	// always return auth fail
	w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
	http.Error(w, "authorization failed", http.StatusUnauthorized)
}

/*

 */
