package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var sshPort string
var mysqlPort string
var httpPort string
var batcherBucket string

// main is the application start point
func main() {

	// Flags are set during testing but env used during lambda runs
	flag.StringVar(&sshPort, "sshport", "", "Enable SSH Server on this port")
	flag.StringVar(&httpPort, "httpport", "", "Enable http Server on this port")
	flag.StringVar(&mysqlPort, "mysqlport", "", "Enable MySQL Server on this port")
	flag.StringVar(&batcherBucket, "batcher-bucket", "", "S3 bucket to sent events to")
	flag.Parse()

	doneChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	var err error
	var atleastonestarted bool

	// setup the batcher to process results
	b := newBatcher(doneChan, batcherBucket, &wg)
	_, err = b.Start()
	if err != nil {
		log.Fatalf("failed to start batcher - err: %v\n", err)
	}

	if sshPort != "" {
		// start the ssh server
		_, err := startSSH(sshPort, b)
		if err != nil {
			log.Fatalf("start ssh failed. err: %v\n", err)
		}
		log.Printf("ssh server started on port %s\n", sshPort)
		atleastonestarted = true
	}

	if mysqlPort != "" {
		// start the mysql server
		_, err := startMySQL(mysqlPort, b)
		if err != nil {
			log.Fatalf("start mysql failed. err: %v\n", err)
		}
		log.Printf("mysql server started on port %s\n", mysqlPort)
		atleastonestarted = true
	}

	if httpPort != "" {
		// start the http server
		_, err := startHttp(httpPort, b)
		if err != nil {
			log.Fatalf("start http failed. err: %v\n", err)
		}
		log.Printf("http server started on port %s\n", httpPort)
		atleastonestarted = true
	}

	// shutting down
	if atleastonestarted == true {
		fmt.Printf("\nShutting down system on signal: %v\n", <-sigChan)
	}
	close(doneChan)
	//sref.Close()

	wg.Wait()
	fmt.Printf("Shutdown complete\n")
}

/*

 */
