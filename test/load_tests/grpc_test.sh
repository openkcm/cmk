#!/bin/bash
k6 run \
  --env BROKER=localhost:9092 \
  --env USE_TLS=false \
  --out json=result_grpc_test.json  \
  grpc_test.js

