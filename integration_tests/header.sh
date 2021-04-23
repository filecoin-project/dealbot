#!/bin/bash

my_dir="$(dirname "$0")"

#set -o errexit ... re-enable when we fix the storage deal bug on Lotus side

# go install dealbot and devnet
pushd $my_dir/../
go install ./...
popd

# set HOME *after* "go install", to not nuke Go's caches
export TEMPDIR=$(mktemp -d)
export HOME=$TEMPDIR

# If the user specified an existing lotus node, don't start devnet.
# We return instead of exiting, since this is a source-d script.
if [[ -z $FULLNODE_API_INFO ]]; then
	function finish {
		kill -9 $DEVNETPID
	}
	trap finish EXIT
	LOTUS_PATH=$TEMPDIR/.lotus
	LOTUS_MINER_PATH=$TEMPDIR/.lotusminer

	devnet &
	DEVNETPID=$!

	# TODO: poll/wait for devnet to be ready
	sleep 60

	export DEALBOT_DATA_DIRECTORY=$TEMPDIR
	export FULLNODE_API_INFO="$(cat $LOTUS_PATH/token):$(cat $LOTUS_PATH/api)"
fi

pushd $TEMPDIR
