package batcher

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type batcher struct {
	batchChan    chan string     // channel to receive results on
	wg           *sync.WaitGroup // wait group to signal when we exit
	doneChan     chan struct{}   // chan to advise system is shutting down
	resultURL    string          // location to post the results
	sharedSecret string          // shared secret to encode with header before sending to remote
	lastPush     time.Time       // time of last remote push
	max          int             // max number of items in a batch
}

// bChan is a reference to the channel to send work on
var bChan chan string

// New creates a new Batcher and configures
func New(doneChan chan struct{}, flagURL string, wg *sync.WaitGroup) *batcher {

	b := &batcher{
		doneChan: doneChan,
		wg:       wg,
		max:      62626, // slightly less than 64K so it is less then one SNS billing unit
		lastPush: time.Now(),
	}

	if b.resultURL = os.Getenv("BATCHER_RESULT_URL"); b.resultURL == "" {
		b.resultURL = flagURL
	}
	return b
}

// Start starts the Batcher running and listening on the bChan for items to post to remote url
func (b *batcher) Start() (chan string, error) {

	if b.sharedSecret = os.Getenv("BATCHER_SS"); b.sharedSecret == "" {
		return nil, errors.New("batcher - can not find shared secret")
	}

	if b.resultURL == "" {
		return nil, errors.New("resultURL not set")
	}

	b.batchChan = make(chan string)

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

	var res string
	var bB bytes.Buffer

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
				// if last result was > 15 minutes ago then push now
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

				// add the result to the batch Buffer
				bB.WriteString(res + "\n")

				if x := bB.Len(); x >= b.max {
					b.pushToRemote(bB)
					loopwhile = false
				}
			}
		} // end loopwhile
	} // end dowhile

	// dropped out of loops so push the last items to remote service
	if x := bB.Len(); x > 0 {
		log.Printf("shutting down so pushing last results...\n")
		b.pushToRemote(bB)
	} else {
		log.Printf("shutting down. No results to push\n")
	}
	// return and defer wg done
}

// pushToRemote will call the remote result microservice and post the result batch
func (b *batcher) pushToRemote(bB bytes.Buffer) {

	b.lastPush = time.Now()

	// generate the md5 checksum
	h := md5.New()
	io.WriteString(h, b.sharedSecret)                        // first add in the shared secret
	io.WriteString(h, bB.String())                           // then add in the body
	myMagic := base64.StdEncoding.EncodeToString(h.Sum(nil)) // and check the sum

	err := b.postResult(myMagic, bB)
	if err != nil {
		log.Printf("batcher postResult error: %v\n", err)
	}
	log.Printf("batcher pushed to remote\n")
}

func (b *batcher) postResult(myMagic string, bB bytes.Buffer) error {

	client := &http.Client{}

	req, err := http.NewRequest("POST", b.resultURL, bytes.NewReader(bB.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Add("x-Batcher-Checksum", myMagic)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("error posting results")
	}
	return nil
}

// AddToBatch creates a goroutine and adds an item to the batcher chan
func AddToBatch(result string) {
	go func(result string) {
		bChan <- result
	}(result)

}

/*


 */
