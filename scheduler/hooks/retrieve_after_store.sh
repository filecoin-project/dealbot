#!/bin/bash
# Usage: bash retrieve_after_store.sh <UUID>
# expected installed env binaries: mktemp, curl, jq
# env vars used:
# DEALBOT_CONTROLLER_ENDPOINT

curl -o /tmp/dealbot.$1 $DEALBOT_CONTROLLER_ENDPOINT/tasks/$1\?parsed

if [ "$(cat /tmp/dealbot.$1 | jq '.StorageTask == null')" = "true" ]; then
  echo "task not a storage task."
  exit 0
fi

if [ "$(cat /tmp/dealbot.$1 | jq '.TimeToLastByteMS == null')" = "true" ]; then
  echo "task did not finish transfer."
  exit 0
fi

MINER=$(cat /tmp/dealbot.$1 | jq .StorageTask.Miner -c)
TAG=$(cat /tmp/dealbot.$1 | jq .StorageTask.Tag -c)
PAYLOAD=$(cat /tmp/dealbot.$1 | jq .PayloadCID -c)

echo "{\"Miner\": $MINER, \"Tag\": $TAG, \"PayloadCID\": \"$PAYLOAD\", \"CarExport\": false, \"Schedule\": \"$(date -v+5M +"%M %H * * *")\", \"ScheduleLimit\": 1}" | jq -c . > /tmp/dealbot.$1.json
curl -X POST -d "@/tmp/dealbot.$1.json" -H "Content-Type: application/json" $DEALBOT_CONTROLLER_ENDPOINT/tasks/retrieval
