#!/bin/bash

# This script runs from cron to check if the instance is scheduled for termination - 2 minute warning
BASEDIR="/autobuild"

# Test at the top of the minute
sleep 4

# check if we are terminating and if not then exit
RES=$(/usr/bin/curl -o /dev/null --silent --head --write-out '%{http_code}\n' http://169.254.169.254/latest/meta-data/spot/termination-time)

if [ "x${RES}" != "x404" ]; then
    # when we are going to terminate then run all scripts in the uninstall dir
    for u in $(ls -1 ${BASEDIR}/uninstall); do
        ${BASEDIR}uninstall/${u}
    done
    exit 0
fi

sleep 20

# test at 20 sec past

RES=$(/usr/bin/curl -o /dev/null --silent --head --write-out '%{http_code}\n' http://169.254.169.254/latest/meta-data/spot/termination-time)

if [ "x${RES}" != "x404" ]; then
    # when we are going to terminate then run all scripts in the uninstall dir
    for u in $(ls -1 ${BASEDIR}/uninstall); do
        ${BASEDIR}uninstall/${u}
    done
    exit 0
fi

sleep 20

# test at 40 sec past the minute
RES=$(/usr/bin/curl -o /dev/null --silent --head --write-out '%{http_code}\n' http://169.254.169.254/latest/meta-data/spot/termination-time)

if [ "x${RES}" != "x404" ]; then
    # when we are going to terminate then run all scripts in the uninstall dir
    for u in $(ls -1 ${BASEDIR}/uninstall); do
        ${BASEDIR}uninstall/${u}
    done
    exit 0
fi

