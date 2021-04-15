#!/bin/bash

my_dir="$(dirname "$0")"

#set -o errexit ... re-enable when we fix the storage deal bug on Lotus side
set -x

export TEMPDIR=$(mktemp -d)
export HOME=$TEMPDIR

# If the user specified an existing lotus node, don't start devnet.
# We return instead of exiting, since this is a source-d script.
if [[ -z $LOTUS_API ]]; then
	function finish {
		kill -9 $DEVNETPID
	}
	trap finish EXIT

	# go install dealbot and devnet
	pushd $my_dir/../
	go install ./...
	popd
	LOTUS_PATH=$TEMPDIR/.lotus
	LOTUS_MINER_PATH=$TEMPDIR/.lotusminer

	devnet &
	DEVNETPID=$!

	sleep 60

	export LOTUS_TOKEN=$(cat $LOTUS_PATH/token)
	export LOTUS_API=$(cat $LOTUS_PATH/api)
fi

pushd $TEMPDIR
