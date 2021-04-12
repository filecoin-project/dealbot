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

dealbot --api $TOKEN:$API storage-deal --data-dir $DATADIR --miner $MINER
