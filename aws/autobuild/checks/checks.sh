#!/bin/bash

# This script is run once after an instance is built and reprts on the results

# first thing is to remove myself so we only run once
rm -f /etc/cron.d/checks-cron

# make sure all is up and running before reporting in
sleep 37

# Gather the output and send it off
{
echo "New Instance Details"
echo
echo "=========================================="
/sbin/ifconfig
echo

echo "=========================================="
/bin/netstat -plntu
echo

echo "=========================================="
/bin/ps fauwx
echo

} | mailx -s "New AWS Spot Instance - $(/bin/hostname)" devnull@example.com

