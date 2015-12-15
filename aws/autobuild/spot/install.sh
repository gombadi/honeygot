#!/bin/bash

# add the spot instance checks cron job.
# When the spot instance is to be terminated then 
# each script in the uninstall directory will be run

cp /autobuild/spot/spot-cron /etc/cron.d/

# cron will run at required time and do work

/usr/bin/logger "autobuild: spot termination check installed"

