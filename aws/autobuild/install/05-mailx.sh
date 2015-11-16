#!/bin/bash

# install the mailx package to enable commandline mail sending
yum install -y mailx

/usr/bin/logger "autobuild: mailx installed"
