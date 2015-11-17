package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gombadi/honeygot/code/honeygot/batcher"
	"github.com/gombadi/honeygot/code/honeygot/httpauth"
	"github.com/gombadi/honeygot/code/honeygot/ssh"
)

// main is the application start point
func main() {

	doneChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	var err error

	// setup the batcher to process results
	b := batcher.New(doneChan, "", &wg)
	_, err = b.Start()
	if err != nil {
		log.Fatalf("failed to start batcher - err: %v\n", err)
	}

	// start the ssh server
	sref := ssh.New()
	err = sref.Start()
	if err != nil {
		log.Fatalf("start ssh failed. err: %v\n", err)
	}

	// start the http auth server
	href := httpauth.New()
	err = href.Start()
	if err != nil {
		log.Fatalf("start ssh failed. err: %v\n", err)
	}

	// shutting down
	fmt.Printf("\nShutting down system on signal: %v\n", <-sigChan)
	close(doneChan)
	sref.Close()

	wg.Wait()
	fmt.Printf("Shutdown complete\n")
	os.Exit(0)
}

/*

*/
