#!/bin/bash

# install & config tor as a bridge
MYCONF="$(dirname ${0})/settings.conf"

if [ -f ${MYCONF} ]; then
        source ${MYCONF}
fi

# throw away output and run in background
#$(dirname ${0})/honeygot-linux64 -httpport 80 -mysqlport 3306 -sshport 22 -batcher-bucket ${BATCHER_S3_BUCKET} > /dev/null 2>&1 &
$(dirname ${0})/honeygot-linux64 -sshport 22 -batcher-bucket ${BATCHER_S3_BUCKET} > /dev/null 2>&1 &

# add a message that will appear in the AWS console output
/usr/bin/logger "autobuild: honeygot installed and started"
