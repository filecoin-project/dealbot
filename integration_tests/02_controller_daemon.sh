#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

export DEALBOT_MINER_ADDRESS=t01000

function finish {
	kill -9 $CONTROLLER_PID $DAEMON_PID $CONTROLLER_TAIL_PID $DAEMON_TAIL_PID
}
trap finish EXIT

dealbot controller --listen="localhost:8085" &>controller.log &
CONTROLLER_PID=$!
CONTROLLER_ENDPOINT="http://localhost:8085"

tail -f controller.log | sed 's/^/controller.log: /' &
CONTROLLER_TAIL_PID=$!

# Give it half a second to start.
sleep 0.5
if ! kill -0 $CONTROLLER_PID; then
	cat controller.log
	unset CONTROLLER_PID # no need to kill it
	exit 1
fi

dealbot daemon --endpoint=$CONTROLLER_ENDPOINT &>daemon.log  &
DAEMON_PID=$!

# Give it half a second to start.
sleep 0.5
if ! kill -0 $DAEMON_PID; then
	cat daemon.log
	unset DAEMON_PID # no need to kill it
	exit 1
fi

tail -f daemon.log | sed 's/^/daemon.log: /' &
DAEMON_TAIL_PID=$!

curl --header "Content-Type: application/json" \
	--request POST \
	--data '{"miner":"t01000","max_price_attofil":100000000000000000,"size":1024,"start_offset":0,"fast_retrieval":true,"verified":false}' \
	"$CONTROLLER_ENDPOINT/tasks/storage"

# TODO: Also test a retrieval task.

# TODO: Once the daemon posts all updates back to the controller, use the
# controller logs alone.

# TODO: Track the status logs more intelligently. Probably too complex for shell.

# TODO: Also test multiple daemons, and e.g. submitting many trivial tasks and
# seeing that every daemon picks up at least one of them.

# On average, the storage deal takes about four minutes.
# Use a timeout of ten minutes, just in case.
for ((i = 0; i < 10*60; i++)); do
	if grep -q StorageDealActive daemon.log; then
		echo "Storage deal is active!"
		exit 0
	fi
	sleep 1
done

# The logs are already being printed out.
echo "Timed out waiting for storage deal to be active."
exit 0
