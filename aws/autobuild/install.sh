#!/bin/bash

AUTOBASE="/autobuild"

# source file which exports all variables
if [ -f ${AUTOBASE}/settings.conf ]; then
    source ${AUTOBASE}/settings.conf
fi

# run all install scripts
for i in $(ls -1 /autobuild/*/install.sh); do
    if [ -x ${i} ]; then
        ${i}
    else
        echo "Warning: unable to run install scripti >> ${i}"
    fi
done

# if the outfile has content then send it off
if [ -f ${OUTFILE} ]; then
    /usr/bin/aws sns publish --region ${AWS_REGION} --subject "Autobuild Output for $(hostname)" --topic-arn ${ARN} --message file://${OUTFILE}
fi

