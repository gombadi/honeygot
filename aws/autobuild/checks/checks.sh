#!/bin/bash

# This script is run once after an instance is built and reprts on the results
MYCONF="$(dirname ${0})/settings.conf"

if [ -f ${MYCONF} ]; then
        source ${MYCONF}
fi

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
} > ${STATUSFILE}

# send the notice out via SNS
/usr/bin/aws sns publish --region ${AWS_REGION} --topic-arn ${ARN} --message file://${STATUSFILE} --subject "Spot instance started - $(/bin/hostname)"

