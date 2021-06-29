#!/bin/bash

CLUSTER_DIR=$1
"$POSTGRES_BIN"/pg_ctl -D "$CLUSTER_DIR" stop