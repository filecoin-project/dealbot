#!/bin/bash

CONTROLLER_ENDPOINT=$1
DATAFILE=$2
if [ -n "$2" ]; then
  DATAFILE=$2
fi

LEN=$(cat $DATAFILE | jq "length")
for (( i=0; i<$LEN; i++))
do
  DATA=$(cat $DATAFILE | jq ".[$i]")
  STORAGE=$(cat $DATAFILE | jq ".[$i].size!=null")
  if [ "$STORAGE" = "true" ]; then
    curl --header "Content-Type: application/json" \
      --request POST \
      --data '$DATA' \
      "$CONTROLLER_ENDPOINT/tasks/storage"
  else
    curl --header "Content-Type: application/json" \
      --request POST \
      --data '$DATA' \
      "$CONTROLLER_ENDPOINT/tasks/retrieval"
  fi
done
