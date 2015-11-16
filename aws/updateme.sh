#!/bin/bash

# This script will create the tar file and upload it to AWS s3 bucket

if [ ! -d autobuild/install ]; then
    echo "Error: Unable to find the autobuild install directory in current location"
    exit 1
fi

if [ -z "$1" ]; then
    echo "Error: No AWS s3 bucket supplied"
    echo "Usage: ./updateme.sh s3://aws-s3-bucket-name/"
    echo
    echo "Note: You must have AWS access keys configured to run this script"
    exit 1
fi

tar -czf autobuild.tar.gz autobuild/

aws s3 cp ./autobuild.tar.gz ${1}


