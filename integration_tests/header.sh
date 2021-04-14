#!/bin/bash

#set -o errexit ... re-enable when we fix the storage deal bug on Lotus side
set -x

err_report() {
    echo "Error on line $1 : $2"
}
#FILENAME=`basename $0`
#trap 'err_report $LINENO $FILENAME' ERR

function finish {
  kill -9 $DEVNETPID
}
trap finish EXIT

# go install dealbot and devnet
my_dir="$(dirname "$0")"
pushd $my_dir/../
go install ./...
popd

TEMPDIR=`mktemp -d`
export HOME=$TEMPDIR
export LOTUS_PATH=$TEMPDIR/.lotus
export LOTUS_MINER_PATH=$TEMPDIR/.lotusminer

pushd $TEMPDIR

devnet &
DEVNETPID=$!

sleep 60
