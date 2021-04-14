#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

export LOTUS_PATH=$TEMPDIR/.lotus
export LOTUS_MINER_PATH=$TEMPDIR/.lotusminer

TOKEN=`cat $LOTUS_PATH/token`
API=`cat $LOTUS_PATH/api`
DATADIR=$TEMPDIR
MINER=t01000

dealbot --api $TOKEN:$API storage-deal --data-dir $DATADIR --miner $MINER

returnValue=$?
if [ $returnValue -eq 0 ]; then
    echo "expected first storage-deal to fail, but it returned exit code 0"
    exit 1
fi

dealbot --api $TOKEN:$API storage-deal --data-dir $DATADIR --miner $MINER

returnValue=$?
if [ $returnValue -ne 0 ]; then
    echo "expected second storage-deal to succeed, but it returned exit code != 0"
    exit 1
fi

CID=`lotus client local | tail -1 | awk '{print $2}'`

dealbot --api $TOKEN:$API retrieval-deal --data-dir $DATADIR --miner $MINER --cid=$CID

returnValue=$?
if [ $returnValue -ne 0 ]; then
    echo "expected retrieval-deal to succeed, but it returned exit code != 0"
    exit 1
fi
