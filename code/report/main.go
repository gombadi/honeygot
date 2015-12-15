package main

/*
To run this it needs to be zipped into a file along with the node.js wrapper and uploaded
to AWS Lambda. It is then run and the output is shown in Cloudwatch logs
*/

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sns"
)

const (
	maxRoutines = 50
)

// command line variables used for testing
var region string    // AWS region to test in
var s3bucket string  // s3 bucket to read the results from
var snsreport string // s3 bucket to write the report to
var prefix string    // s3 bucket prefix to read the results from. i.e. what days results to read
var debug bool

func main() {

	ts := time.Now()
	t1 := time.Now()
	fmt.Printf("Time: Process start at - %s\n", ts)
	// Flags are set during testing but env used during lambda runs
	flag.StringVar(&region, "region", "", "AWS region to use")
	flag.StringVar(&s3bucket, "s3bucket", "", "AWS s3 Bucket to read results from")
	flag.StringVar(&snsreport, "snsreport", "", "AWS sns arn topic to publish report to")
	flag.StringVar(&prefix, "prefix", "", "s3 Bucket prefix to use")
	flag.BoolVar(&debug, "debug", false, "Print report local instead of via SNS")
	flag.Parse()

	// get report date which is yesterday
	yr, mth, day := time.Now().UTC().AddDate(0, 0, -1).Date()

	if prefix == "" {
		prefix = fmt.Sprintf("%v-%v/%v/", yr, int(mth), day)
	}
	sess := session.New()
	s3svc := s3.New(sess)

	// currently only get the first 999 files in the s3 bucket
	params := &s3.ListObjectsInput{
		Bucket:  aws.String(s3bucket),
		MaxKeys: aws.Int64(999),
		Prefix:  aws.String(prefix),
	}
	listObjResp, err := s3svc.ListObjects(params)
	if err != nil {
		fmt.Printf("aws err - listobjects: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Time: received s3 file list - %v\n", time.Since(t1).String())
	t2 := time.Now()

	aeMap, err := fillMap(s3svc, listObjResp.Contents)
	if err != nil {
		fmt.Printf("error reading data from s3: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Time: read all files from s3 - %v\n", time.Since(t2).String())
	t3 := time.Now()

	report := produceReport(aeMap)

	fmt.Printf("Time: report produced - %v\n", time.Since(t3).String())

	report.WriteString(fmt.Sprintf("\n========\nReport Stats:\nTime to get s3 file list: %v\n", t2.Sub(t1).String()))
	report.WriteString(fmt.Sprintf("Time to download all s3 files: %v\nTime to produce report: %v\n\n", t3.Sub(t2).String(), time.Since(t3).String()))

	ro := fmt.Sprintf("Final Report:\nNumber of files processed: %d\n\n%s\n", len(listObjResp.Contents), report.String())
	ms := fmt.Sprintf("Honeygot Daily Report for %d-%d-%d", yr, int(mth), day)

	t4 := time.Now()

	if debug == false && snsreport != "" {
		snsparams := &sns.PublishInput{
			Message:  aws.String(ro), // Report output
			Subject:  aws.String(ms), // Message Subject for emails
			TopicArn: aws.String(snsreport),
		}

		// push the report to SNS for distribution
		snssvc := sns.New(sess)
		publishResp, err := snssvc.Publish(snsparams)
		if err != nil {
			fmt.Printf("error publishing to AWS SNS: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("Published to AWS SNS: %s\n", *publishResp.MessageId)
		}
	} else {
		fmt.Printf("Debug output -\nSubject: %s\nReport -\n", ms)
		fmt.Printf("%s\n", ro)
	}

	fmt.Printf("Time: push to SNS - %v\n", time.Since(t4).String())
	fmt.Printf("Time: report complete - %v\n", time.Since(ts).String())

}

// produceReport creates a bytesBuffer with the text of the final report
func produceReport(aeMap map[string]*AuthEvent) bytes.Buffer {

	var bB bytes.Buffer

	srcIPMap := make(map[string]int)
	destIPMap := make(map[string]int)
	userMap := make(map[string]int)
	pwMap := make(map[string]int)
	authMap := make(map[string]int)

	for _, v := range aeMap {

		authMap[v.AuthType]++
		if v.AuthType != "sshPass" {
			continue
		}

		// increment counters for each selection
		srcIPMap[v.SrcIP]++
		destIPMap[v.DestIP]++
		userMap[v.User]++
		pwMap[v.Credentials]++
	}

	sortedsrsIP := rankByTopMax(srcIPMap, 25)
	sorteddestIP := rankByTopMax(destIPMap, 25)
	sortedUsers := rankByTopMax(userMap, 25)
	sortedpwList := rankByTopMax(pwMap, 25)
	sortedauth := rankByTopMax(authMap, 25)

	bB.WriteString(fmt.Sprintf("Total auth events: %d\n\n", len(aeMap)))

	bB.WriteString(fmt.Sprintf("Source IP address:\nTotal different IPs: %d\nTop 25 source addresses -\n", len(srcIPMap)))
	bB.WriteString(fmt.Sprintf("Count\tSrc IP\n"))
	for _, v := range sortedsrsIP {
		bB.WriteString(fmt.Sprintf("%v\t%v\n", v.Value, v.Key))
	}
	bB.WriteString("========================\n\n")

	bB.WriteString(fmt.Sprintf("Destination IP address:\nTotal different IPs: %d\nTop 25 destination addresses -\n", len(destIPMap)))
	bB.WriteString(fmt.Sprintf("Count\tSrc IP\n"))
	for _, v := range sorteddestIP {
		bB.WriteString(fmt.Sprintf("%v\t%v\n", v.Value, v.Key))
	}
	bB.WriteString("========================\n\n")

	bB.WriteString(fmt.Sprintf("User names used:\nTotal different usernames: %d\nTop 25 usernames -\n", len(userMap)))
	bB.WriteString(fmt.Sprintf("Count\tSrc IP\n"))
	for _, v := range sortedUsers {
		bB.WriteString(fmt.Sprintf("%v\t%v\n", v.Value, v.Key))
	}
	bB.WriteString("========================\n\n")

	bB.WriteString(fmt.Sprintf("Credentials used:\nTotal different credentials: %d\nTop 25 credentials -\n", len(pwMap)))
	bB.WriteString(fmt.Sprintf("Count\tSrc IP\n"))
	for _, v := range sortedpwList {
		bB.WriteString(fmt.Sprintf("%v\t%v\n", v.Value, v.Key))
	}
	bB.WriteString("========================\n\n")

	bB.WriteString(fmt.Sprintf("Auth types:\nTotal different authTypes: %d\nTop 25 authTypes -\n", len(authMap)))
	bB.WriteString(fmt.Sprintf("Count\tType\n"))
	for _, v := range sortedauth {
		bB.WriteString(fmt.Sprintf("%v\t%v\n", v.Value, v.Key))
	}
	bB.WriteString("========================\n\n")

	return bB
}

// fillMap starts goroutines to download files from s3 and produce the map with all events
func fillMap(s3svc *s3.S3, s3Files []*s3.Object) (map[string]*AuthEvent, error) {
	fileChan := make(chan string, maxRoutines)
	resChan := make(chan *AuthEvent)
	doneChan := make(chan struct{})

	tm := make(map[string]*AuthEvent)
	var wg sync.WaitGroup

	if debug == true {
		fmt.Printf("debug: start goroutines to pull from s3\n")
	}

	for x := 0; x <= maxRoutines; x++ {
		wg.Add(1)
		go processS3File(fileChan, resChan, &wg)
	}

	// start the results goroutine running
	go processResults(resChan, doneChan, tm)

	if debug == true {
		fmt.Printf("debug: start pushing filenames into fileChan\n")
	}

	// push all s3 file names into the chan. May block before sending all
	for _, s3File := range s3Files {
		if debug == true {
			fmt.Printf("debug: pushing %s\n", *s3File.Key)
		}
		fileChan <- *s3File.Key
	}

	if debug == true {
		fmt.Printf("debug: closing fileChan and waiting for goroutines to end\n")
	}

	// all filenames are in the chan so close it and let the goroutines drain it
	close(fileChan)

	// wait for the goroutines to do their work and exit
	wg.Wait()

	if debug == true {
		fmt.Printf("debug: closing resChan and waiting for doneChan\n")
	}

	// this will exit the results processor
	close(resChan)
	// wait for the results to shutdown then return for reporting
	<-doneChan
	close(doneChan)

	return tm, nil
}

// processS3File runs in a goroutine and reads filenames from the fileChan, pulls down the file, extracts the events
// and then sends then on the resChan
func processS3File(fileChan chan string, resChan chan *AuthEvent, wg *sync.WaitGroup) {

	defer wg.Done()

	s3svc := s3.New(session.New())
	var b bytes.Buffer // A Buffer needs no initialization.

	// read filenames from chan until it is closed
	for s3FileName := range fileChan {

		if debug == true {
			fmt.Printf("getting file from s3: %s\n", s3FileName)
		}

		params := &s3.GetObjectInput{
			Bucket: aws.String(s3bucket),   // Required
			Key:    aws.String(s3FileName), // Required
		}
		getObjOutput, err := s3svc.GetObject(params)
		if err != nil {
			fmt.Printf("s3.GetObject err - file: %s err: %v\n", s3FileName, err)
			continue
		}

		b.ReadFrom(getObjOutput.Body)
		authEvents := bytes.Split(b.Bytes(), []byte("\n"))

		for _, jsonae := range authEvents {
			// last \n does not need unmarshal
			if len(jsonae) < 10 {
				continue
			}
			ae := &AuthEvent{}
			err := json.Unmarshal(jsonae, &ae)
			if err != nil {
				fmt.Printf("unmarshal fail:%v\n", err)
				fmt.Printf("unmarshal json:\n%s\n", jsonae)
			} else {
				resChan <- ae
			}
		}
		// reset to a clean buffer for each file
		b.Reset()
	}
}

// processResults runs in a goroutine and reads results from the workers and updates a global map
func processResults(resChan chan *AuthEvent, doneChan chan struct{}, tm map[string]*AuthEvent) {

	// read things from the resChan and add to the global map
	for ae := range resChan {
		tm[ae.Hash] = ae
	}

	// tell the world we are done
	doneChan <- struct{}{}
}

/*

 */
