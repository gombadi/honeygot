#!/bin/bash

# install & config tor as a bridge
MYDIR=/autobuild/honeygot

cd ${MYDIR}

# set the environment for honeygot to run
export BATCHER_SS="thereshouldbesomethinghere"
export BATCHER_RESULT_URL="http://someurl.example.com:1234/"

export HONEYGOT_SSHPORT=22
export HONEYGOT_HTTPPORT=80

# throw away output and run in background
./honeygot > /dev/null 2>&1 &

# add a message that will appear in the AWS console output
/usr/bin/logger "autobuild: honeygot installed and started"
