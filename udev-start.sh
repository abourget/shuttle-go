#!/bin/bash

export HOME=/home/abourget

# Only the "event" input device, not the "mouse" device (where MINOR=34)
if [ $MINOR != "79" ]; then exit 99; fi

#LOGFILE=/tmp/shuttle-`basename $DEVNAME`.env
#env > $LOGFILE

export XAUTHORITY=$HOME/.Xauthority
export DISPLAY=:0.0
export PATH=/usr/bin  # which includes the path to `xdotool`

$HOME/go/bin/shuttle-go -config $HOME/.shuttle-go.json -log-file /tmp/shuttle-go.log $DEVNAME &
