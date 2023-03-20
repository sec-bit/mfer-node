#!/bin/sh

# Load environment variables from .env file if it exists
if [ -f .env ]; then
  source .env
fi

# Check if environment variables are defined, otherwise use default values and append flag if LOGPATH is empty
UPSTREAM=${UPSTREAM:+"--upstream=$UPSTREAM"}
LISTEN=${LISTEN:+"--listen=$LISTEN"}
MAXKEYS=${MAXKEYS:+"--maxkeys=$MAXKEYS"}
BATCHSIZE=${BATCHSIZE:+"--batchsize=$BATCHSIZE"}
LOGPATH_FLAG="--logpath="
LOGPATH=${LOGPATH_FLAG}${LOGPATH:-""}

# Build command with environment variables
command="$UPSTREAM $LISTEN $MAXKEYS $BATCHSIZE $LOGPATH"
# Run command
mfer-node $command