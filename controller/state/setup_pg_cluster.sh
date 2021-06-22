#!/bin/bash

CLUSTER_DIR=$1
CLUSTER_PORT=$2

"$POSTGRES_BIN"/initdb -U postgres "$CLUSTER_DIR"
mv "$CLUSTER_DIR/postgresql.conf" "$CLUSTER_DIR/postgresql.conf.orig"
sed "s/#port = 5432/port = $CLUSTER_PORT/g" "$CLUSTER_DIR/postgresql.conf.orig" > "$CLUSTER_DIR/postgresql.conf" 
"$POSTGRES_BIN"/pg_ctl -D "$CLUSTER_DIR" start