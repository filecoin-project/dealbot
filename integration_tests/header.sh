#!/bin/bash

my_dir="$(dirname "$0")"

#set -o errexit ... re-enable when we fix the storage deal bug on Lotus side

# go install dealbot and devnet
pushd $my_dir/../
go install ./...
popd

# set HOME *after* "go install", to not nuke Go's caches
export TEMPDIR=$(mktemp -d --suffix="-dealbot-test")
export HOME=$TEMPDIR

pushd $TEMPDIR

# If the user specified an existing lotus node, don't start devnet.
# We return instead of exiting, since this is a source-d script.
if [[ -z $FULLNODE_API_INFO ]]; then
	function stop_devnet {
		# Just a SIGTERM, to let it clean up lotus sub-processes.
		kill $DEVNETPID

		# Print the tail of both logs, for the sake of debugging.
		for logf in lotus-daemon.log lotus-miner.log; do
			echo $logf
			tail -n 20 $TEMPDIR/$logf
		done
	}
	trap stop_devnet EXIT
	LOTUS_PATH=$TEMPDIR/.lotus
	LOTUS_MINER_PATH=$TEMPDIR/.lotusminer

	devnet &
	DEVNETPID=$!

	sleep 60

	# We also wait for the APIs to be up before using them.
	# The commands print every second by default, which is too verbose.

	echo "Waiting for the default wallet to be set up..."

	lotus wait-api | grep -v 'Not online yet'
	for ((i = 0; ; i++)); do
		sleep 2
		if lotus wallet default; then
			break
		fi
		if ((i > 30)); then
			echo "wallet not ready after 60s"
			exit 1
		fi
	done

	echo "Waiting for the miner to have one peer..."

	lotus-miner wait-api | grep -v 'Not online yet'
	for ((i = 0; ; i++)); do
		sleep 2
		if lotus-miner net peers | grep tcp; then
			break
		fi
		if ((i > 30)); then
			echo "peer not ready after 60s"
			exit 1
		fi
	done

	# We'd sometimes still get weird deal errors after the waits above, like:
	#
	#    deal rejected: proposed provider collateral above maximum: 415958 > 0
	#
	# Waiting a bit longer seems to avoid this.
	# TODO: figure out what else we need to wait for.
	sleep 10

	export DEALBOT_DATA_DIRECTORY=$TEMPDIR
	export FULLNODE_API_INFO="$(cat $LOTUS_PATH/token):$(cat $LOTUS_PATH/api)"
fi
