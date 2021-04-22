#!/bin/bash

CONTROLLER_ENDPOINT=$1

LEN=$(cat ./sample_tasks.json | jq "length")
for (( i=0; i<$LEN; i++))
do
  DATA=$(cat ./sample_tasks.json | jq ".[$i]")
  STORAGE=$(cat ./sample_tasks.json | jq ".[$i].size!=null")
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
  done
done
