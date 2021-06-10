#!/bin/bash

# COMMENT TO ENABLE
echo "${0}: test skipped"
exit 0

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

export DEALBOT_MINER_ADDRESS=t01000

function stop_dealbot_and_devnet {
	kill -9 $CONTROLLER_PID $DAEMON_PID $CONTROLLER_TAIL_PID $DAEMON_TAIL_PID

	# We replace devnet's EXIT trap, so run it too.
	stop_devnet
}
trap stop_dealbot_and_devnet EXIT

CONTROLLER_LISTEN="localhost:8085"
CONTROLLER_ENDPOINT="http://${CONTROLLER_LISTEN}"

dealbot controller --listen="${CONTROLLER_LISTEN}" &>dealbot-controller.log &
CONTROLLER_PID=$!

tail -f dealbot-controller.log | sed 's/^/dealbot-controller.log: /' &
CONTROLLER_TAIL_PID=$!

# Give it half a second to start.
sleep 0.5
if ! kill -0 $CONTROLLER_PID; then
	tail -n 50 dealbot-controller.log
	unset CONTROLLER_PID # no need to kill it
	exit 1
fi

dealbot daemon --endpoint=$CONTROLLER_ENDPOINT &>dealbot-daemon.log  &
DAEMON_PID=$!

# Give it half a second to start.
sleep 0.5
if ! kill -0 $DAEMON_PID; then
	tail -n 50 dealbot-daemon.log
	unset DAEMON_PID # no need to kill it
	exit 1
fi

tail -f dealbot-daemon.log | sed 's/^/dealbot-daemon.log: /' &
DAEMON_TAIL_PID=$!

curl --header "Content-Type: application/json" \
	--request POST \
	--data '{"Miner":"t01000","MaxPriceAttoFIL":100000000000000000,"Size":1024,"StartOffset":0,"FastRetrieval":true,"Verified":false,"Schedule":"*/6 * * * *","ScheduleLimit":"20m"}' \
	"$CONTROLLER_ENDPOINT/tasks/storage"

# On average, the storage deal takes about four minutes.
#
# Allowing 6 minutes for each deal to complete, running scheduled deal for a
# total of 20m, timing out after 30 minutes.
prev_active="0"
active="0"
for ((i = 0; i < 30*60; i++)); do
	active=$(grep -c StorageDealActive dealbot-daemon.log)
	if [ "$active" != "$prev_active" ]; then
		echo "Storage deal is active! Deals active: $active"
        if [ "$active" = "2" ]; then
            # Stop test once deal runs again
            break
        fi
		prev_active="$active"
	fi
	sleep 1
done

if [ "$active" != "0" ]; then
	echo "Completed $active storage deals"
	exit 0
fi

# The logs are already being printed out.
echo "Timed out waiting for storage deal to be active."
exit 1
