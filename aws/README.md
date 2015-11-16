# honeygot
AWS based, Golang coded, Honeypot system

## Purpose

This directory contains the files needed to autobuild an AWS Spot instance to run the honeygot system.

## Install


- Edit the files in the autobuild directory to install and configure the packages/applications you want.
- Update the user-data.txt file to use the correct s3 bucket
- Create the Spot request and paste in the user-data.txt file contents

