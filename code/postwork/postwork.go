package main

import (
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var processChan chan *workunit // chan to send all accepted post requests
var ss string                  // shared secret with remote clients
var dir string                 // path to store the received results
var prefix string

type workunit struct {
	rip    string // Remote ip address
	prefix string // prefix to add to a filename
	body   []byte // contents to be written to a file
}

func main() {

	var port string
	var needFlags bool

	if port = os.Getenv("HTTP_PLATFORM_PORT"); port == "" {
		flag.StringVar(&port, "p", "8766", "Port to listen on. Default port: 8766")
		needFlags = true
	}

	if ss = os.Getenv("BATCHER_SS"); ss == "" {
		flag.StringVar(&ss, "s", "thereshouldbesomethinghere", "Shared Secret with client")
		needFlags = true
	}

	if dir = os.Getenv("BATCHER_DIR"); dir == "" {
		flag.StringVar(&dir, "d", "/", "Location to store results")
		needFlags = true
	}

	if prefix = os.Getenv("BATCHER_PREFIX"); prefix == "" {
		flag.StringVar(&prefix, "prefix", "batcher-", "prefix to prefix all saved files")
		needFlags = true
	}

	if needFlags {
		flag.Parse()
	}

	processChan = make(chan *workunit)
	// start the goroutine to listen on the channel & write to disk
	go processPosts(processChan)

	http.HandleFunc("/", postWorkHandler)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// extractIP extracts the ip & port from the http.Request.RemoteAddr field.
// This field is in different formats depending on ipv4/ipv6 and if the
// port info is available
func extractIP(r *http.Request) (ipaddr, port string) {

	// First check if there is a header X-Forwarded-For or similar
	for k, v := range r.Header {
		if ok := strings.Contains(strings.ToLower(k), "x-forwarded-for"); ok == true {
			// only return the first or only ip address
			return strings.Split(v[0], ",")[0], ""
		}
	}

	ipaddr, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", ""
	}
	return ipaddr, port
}

// postWorkHandler processes all incoming requests and saves the posted data to a file
func postWorkHandler(w http.ResponseWriter, r *http.Request) {

	rip, _ := extractIP(r)

	// only process post requests
	if r.Method != "POST" {
		http.Error(w, http.StatusText(404), 404)
		return
	}

	// check magic header present or err 404
	magic := r.Header.Get("x-Batcher-Checksum")
	if magic == "" {
		http.Error(w, http.StatusText(404), 404)
		return
	}

	// get post body & close
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		log.Printf("error reading request body: %v\n", err)
		return
	}

	// check post body matches header
	h := md5.New()
	io.WriteString(h, ss)                                    // first add in the shared secret
	io.WriteString(h, string(body))                          // then add in the body
	myMagic := base64.StdEncoding.EncodeToString(h.Sum(nil)) // and check the sum

	if myMagic != magic {
		http.Error(w, http.StatusText(500), 500)
		log.Printf("magic mismatch - received: %v computed: %v\n", magic, myMagic)
		return
	}

	// pass post body to channel
	go func() {
		wu := &workunit{
			prefix: prefix,
			body:   body,
			rip:    rip,
		}

		processChan <- wu
	}()
}

// processPosts runs in a goroutine and listens on a channel for data to save to file
func processPosts(processChan chan *workunit) {
	var err error

	// process all items on the processChan
	for wu := range processChan {
		if err = dumpIt(wu); err != nil {
			log.Printf("error writing results to file: %v\n", err)
		}
	}

}

// dumpIt creates a file and dumps the received results to it
func dumpIt(wu *workunit) error {

	yr, mth, day := time.Now().Date()
	hr, min, _ := time.Now().Clock()

	file, err := ioutil.TempFile(dir, fmt.Sprintf("%v-%v-%v-%v-%v-%s-", yr, int(mth), day, hr, min, wu.rip))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(wu.body)

	log.Printf("created file: %s\n", file.Name())

	// tell gc we no longer need this
	wu = nil

	return err
}

/*

*/
