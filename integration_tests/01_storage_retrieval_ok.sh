#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

export DEALBOT_DATA_DIRECTORY=$TEMPDIR
export DEALBOT_MINER_ADDRESS=t01000

dealbot storage-deal

returnValue=$?
if [[ $returnValue -ne 0 ]]; then
	echo "expected storage-deal to succeed, but it returned exit code != 0"
	exit 1
fi

CID=$(lotus client local | tail -1 | awk '{print $2}')

dealbot retrieval-deal --cid=$CID

returnValue=$?
if [[ $returnValue -ne 0 ]]; then
	echo "expected retrieval-deal to succeed, but it returned exit code != 0"
	exit 1
fi
