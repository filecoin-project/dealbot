#!/bin/bash

CONTROLLER_ENDPOINT=$1

curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"miner":"t01000","payload_cid":"bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36","car_export":false}' \
  "$CONTROLLER_ENDPOINT/tasks/retrieval"
  
curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"miner":"t01000","payload_cid":"bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg","car_export":false}' \
  "$CONTROLLER_ENDPOINT/tasks/retrieval"

# note: this task will not succeed on the local devnet, as this is a miner on a different network.
curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"miner":"f0127896","payload_cid":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","car_export":false}' \
  "$CONTROLLER_ENDPOINT/tasks/retrieval"

curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"miner":"t01000","max_price_attofil":"100000000000000000","size":1024,"start_offset":0,"fast_retrieval":true,"verified":false}' \
  "$CONTROLLER_ENDPOINT/tasks/storage"
