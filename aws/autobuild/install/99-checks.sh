#!/bin/bash

# post startup checks

cp /autobuild/checks/checks-cron /etc/cron.d/

# cron will run at required time and report on the status
# of this instance

