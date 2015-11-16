#!/bin/bash

# update ssh config

cp /autobuild/sshd/sshd_config /etc/ssh

/sbin/service sshd restart

/usr/bin/logger "autobuild: sshd config updated and restarted"

