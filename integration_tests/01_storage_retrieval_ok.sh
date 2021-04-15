#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

DATADIR=$TEMPDIR
MINER=t01000

dealbot --api=$LOTUS_TOKEN:$LOTUS_API storage-deal --data-dir=$DATADIR --miner=$MINER

returnValue=$?
if [[ $returnValue -ne 0 ]]; then
	echo "expected storage-deal to succeed, but it returned exit code != 0"
	exit 1
fi

CID=$(lotus client local | tail -1 | awk '{print $2}')

dealbot --api=$LOTUS_TOKEN:$LOTUS_API retrieval-deal --data-dir=$DATADIR --miner=$MINER --cid=$CID

returnValue=$?
if [[ $returnValue -ne 0 ]]; then
	echo "expected retrieval-deal to succeed, but it returned exit code != 0"
	exit 1
fi
