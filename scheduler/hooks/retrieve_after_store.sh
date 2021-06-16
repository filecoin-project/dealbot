#!/bin/bash
# Usage: bash retrieve_after_store.sh <UUID>
# expected installed env binaries: mktemp, curl, jq
# env vars used:
# DEALBOT_CONTROLLER_ENDPOINT

curl -o /tmp/dealbot.$1 $DEALBOT_CONTROLLER_ENDPOINT/tasks/$1?parsed

if [[ $(jq '.StorageTask == null' /tmp/dealbot.$1) == "true" ]]; then
  echo "task not a storage task."
  exit 0
fi
if [[ $(jq '.TimeToLastByteMS == null' /tmp/dealbot.$1) == "true" ]]; then
 echo "task did not finish transfer."
  exit 0
fi

MINER=$(cat /tmp/dealbot.$1 | jq .StorageTask.Miner -c)
TAG=$(cat /tmp/dealbot.$1 | jq .StorageTask.Tag -c)
PAYLOAD=$(cat /tmp/dealbot.$1 | jq .PayloadCID -c)

curl -X POST -d - -H "Content-Type: application/json" $DEALBOT_CONTROLLER_ENDPOINT/tasks/retrieval <<EOF
{
  "Miner": $MINER,
  "Tag": $TAG,
  "PayloadCID": "$PAYLOAD",
  "CarExport": false,
  "Schedule": "$(date -d "+5 days" +"%M %H %d %m *")",
  "ScheduleLimit": 1
}
EOF
