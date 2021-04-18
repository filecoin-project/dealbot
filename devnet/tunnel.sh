#!/bin/bash

ADDRESS=ubuntu@ec2-3-83-188-192.compute-1.amazonaws.com

# Start the tunnel to port forward the lotus API on :1234.
# This is a no-op if the port is already running.
echo "Connecting tunnel..."
ssh -N -f -L 1234:localhost:1234 $ADDRESS

# Obtain the api multiaddress and token.
# Note that we need sudo to read the token.
# Also note that $LOTUS_PATH is only set for the non-root user.
# We use awk instead of cat since the files have no trailing newlines.
echo "Collecting lotus api address and token..."
VARS=($(ssh -T $ADDRESS 'sudo awk 1 $LOTUS_PATH/token $LOTUS_PATH/api'))
export FULLNODE_API_INFO="${VARS[0]}:${VARS[1]}"
