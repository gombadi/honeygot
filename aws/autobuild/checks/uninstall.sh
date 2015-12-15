#!/bin/bash

# This script is run once after an instance is built and reports on the results

MYCONF="$(dirname ${0})/settings.conf"

if [ -f ${MYCONF} ]; then
        source ${MYCONF}
fi

# send the notice out via SNS
/usr/bin/aws sns publish --region ${AWS_REGION} --topic-arn ${ARN} --subject "Spot instance $(/bin/hostname) shutting down" --message "$(uptime)" 

