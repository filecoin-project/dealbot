#!/bin/bash

# AWS machine running a nerpanet node.
# Note that deals will be as slow as mainnet.
# ADDRESS=ubuntu@ec2-3-83-188-192.compute-1.amazonaws.com
# LOTUS_PATH=/var/lib/lotus # the system default

# AWS machine running devnet.
ADDRESS=ubuntu@ec2-3-237-19-14.compute-1.amazonaws.com
LOTUS_PATH=/tmp/devnet-lotus # as per README.md

# Start the tunnel to port forward the lotus API on :1234.
# This is a no-op if the port is already running.
echo "Connecting tunnel..."
ssh -N -f -L 1234:localhost:1234 $ADDRESS || exit 1

# Make sure the REMOTE_SSHFS directory exists on the server.
echo "Mounting sshfs temp dir..."
REMOTE_SSHFS=/tmp/dealbot-sshfs
ssh -T $ADDRESS "mkdir -p $REMOTE_SSHFS"

# Create the local directory, and mount.
LOCAL_SSHFS=$(mktemp -d)
sshfs $ADDRESS:$REMOTE_SSHFS $LOCAL_SSHFS || exit 1
export DEALBOT_NODE_DATA_DIRECTORY=$REMOTE_SSHFS
export DEALBOT_DATA_DIRECTORY=$LOCAL_SSHFS

# Obtain the api multiaddress and token.
# Note that we need sudo to read the token.
# We use awk instead of cat since the files have no trailing newlines.
echo "Collecting lotus api address and token..."
API=$(ssh -T $ADDRESS "sudo cat $LOTUS_PATH/api")
TOKEN=$(ssh -T $ADDRESS "sudo cat $LOTUS_PATH/token")
export FULLNODE_API_INFO="${TOKEN}:${API}"
