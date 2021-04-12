#!/bin/bash

set -o errexit
set -x
set -e

err_report() {
    echo "Error on line $1 : $2"
}
FILENAME=`basename $0`
trap 'err_report $LINENO $FILENAME' ERR

function finish {
  kill -9 $DEVNETPID
}
trap finish EXIT

TEMPDIR=`mktemp -d`
export HOME=$TEMPDIR
export LOTUS_PATH=$TEMPDIR/.lotus
export LOTUS_MINER_PATH=$TEMPDIR/.lotusminer

pushd $TEMPDIR

devnet &
DEVNETPID=$!

sleep 30
