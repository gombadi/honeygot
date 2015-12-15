package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type batcher struct {
	batchChan chan *AuthEvent // channel to receive results on
	wg        *sync.WaitGroup // wait group to signal when we exit
	doneChan  chan struct{}   // chan to advise system is shutting down
	extIP     string          // external ip of system to add to all events
	s3Bucket  string          // s3 bucket to upload result batches to
	lastPush  time.Time       // time of last remote push
	max       int             // max number of items in a batch
}

// bChan is a reference to the channel to send work on
var bChan chan *AuthEvent
var extIP string

// newBatcher creates a new Batcher and configures
func newBatcher(doneChan chan struct{}, flagURL string, wg *sync.WaitGroup) *batcher {

	b := &batcher{
		doneChan: doneChan,
		wg:       wg,
		max:      62626, // max batch size to be less than 64k
		lastPush: time.Now(),
	}

	if b.s3Bucket = os.Getenv("BATCHER_S3_BUCKET"); b.s3Bucket == "" {
		b.s3Bucket = flagURL
	}

	// load external ip into global to be used if needed
	extIP = b.getExtIP()

	return b
}

// Start starts the Batcher running and listening on the bChan for items to post to remote url
func (b *batcher) Start() (chan *AuthEvent, error) {

	if b.s3Bucket == "" {
		return nil, errors.New("s3Bucket not set")
	}

	b.batchChan = make(chan *AuthEvent)

	// save a reference to the batch chan in global variable so it can be retrived
	// by AddToBatch
	bChan = b.batchChan

	if b.wg != nil {
		b.wg.Add(1)
	}
	// create the results chan and start the goroutine to listen on it
	go b.run()

	return b.batchChan, nil
}

// run runs in a goroutine and pulls results off the batchChan
// gathers up into batches and then sends to remote microservice
func (b *batcher) run() {

	var bB bytes.Buffer
	var res *AuthEvent

	if b.wg != nil {
		defer b.wg.Done()
	}

	ping := time.NewTicker(time.Minute * 1)

	// loop
	loopwhile := true
	dowhile := true
	for dowhile == true {

		loopwhile = true
		// remove all content from the buffer and start loading more
		bB.Reset()

		for loopwhile == true {
			// wait for a result in the batchChan
			select {
			case <-ping.C:
				// if last result was > 5 minutes ago then push now
				if (time.Now().Unix() - b.lastPush.Unix()) > 1500 {
					if x := bB.Len(); x > 0 {
						b.pushToRemote(bB)
						loopwhile = false
					}
				}
			case <-b.doneChan:
				dowhile = false
				loopwhile = false
			case res = <-b.batchChan:

				// encode to json and write to byteBuffer
				je, err := json.Marshal(res)
				if err == nil {
					bB.Write(je)
					bB.Write([]byte("\n"))
				}
				if x := bB.Len(); x >= b.max {
					b.pushToRemote(bB)
					loopwhile = false
				}
			}
		}
	}
	// dropped out of loop so push the last items to remote service
	if x := bB.Len(); x > 0 {
		log.Printf("shutting down so pushing last results...\n")
		b.pushToRemote(bB)
	} else {
		log.Printf("shutting down. No results to push\n")
	}
	// return and defer wg done
}

// pushToRemote will call the remote result microservice and post the result batch
//func (b *batcher) pushToRemote(results []string) {
func (b *batcher) pushToRemote(bB bytes.Buffer) {

	b.lastPush = time.Now()

	yr, mth, day := time.Now().Date()

	// generate the md5 checksum which is the s3 filename
	h := md5.New()
	io.WriteString(h, string(bB.Bytes()))

	fileName := fmt.Sprintf("%v-%v/%v/honeygot-%x", yr, int(mth), day, h.Sum(nil))

	if len(b.s3Bucket) > 1 {
		// push to s3. Pull all session data from the environment or IAM role
		sess := session.New()
		svc := s3.New(sess)

		params := &s3.PutObjectInput{
			Bucket: aws.String(b.s3Bucket), // Required
			Key:    aws.String(fileName),   // Required
			Body:   bytes.NewReader([]byte(bB.Bytes())),
		}
		_, err := svc.PutObject(params)
		if err != nil {
			log.Printf("batcher postResult error: %v\n", err)
		} else {
			log.Printf("batcher pushed to s3: s3://%s/%s\n", b.s3Bucket, fileName)
		}
	} else {
		log.Printf("warning - unable to push to s3 as no bucket name available. records lost\n")
	}
}

// addevent creates a goroutine and adds an item to the batcher chan
func (b *batcher) addEvent(result *AuthEvent) {
	if x := len(result.Hash); x < 3 {
		result.updateHash()
	}

	go func(result *AuthEvent) {
		bChan <- result
	}(result)

}

// AddToBatch creates a goroutine and adds an item to the batcher chan
func addToBatch(result *AuthEvent) {
	if x := len(result.Hash); x < 3 {
		result.updateHash()
	}

	go func(result *AuthEvent) {
		bChan <- result
	}(result)

}

func (b *batcher) getExtIP() string {

	var extURL, ip string
	if extURL = os.Getenv("HONEYGOT_EXTURL"); extURL != "" {

		// get our external ip address so we can add it to the results
		resp, err := http.Get(extURL)
		if err != nil {
			return ""
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return ""
		}
		resp.Body.Close()

		// extIP is global as it is required in the auth callback functions that
		// are not part of this struct
		ip = strings.TrimSpace(string(body))
		log.Printf("detected external ip as %s\n", ip)
	} else {
		log.Printf("unable to detect external ip address\n")
	}
	return ip
}

/*


 */
