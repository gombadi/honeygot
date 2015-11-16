#!/bin/bash

# shutting down so notify
echo "I am shutting down" | /bin/mail -s "AWS Instance shutdown $(/bin/hostname)" devnull@example.com

