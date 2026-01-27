#!/bin/bash
k6 run \
  --env BROKER=localhost:9092 \
  --env USE_TLS=true \
  --env CA_FILE=./certs/ca.crt \
  --env CERT_FILE=./certs/client.crt \
  --env KEY_FILE=./certs/client.key \
  --out json=result_grpc_test.json  \
  grpc_test.js

