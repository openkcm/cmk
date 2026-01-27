import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';
import { randomSeed } from 'k6';

const client = new grpc.Client();
// Load proto from vendor directory (source of truth) or fallback to local copy
// The vendor directory is populated by 'go mod vendor' command
const protoPath = __ENV.PROTO_PATH || '../../vendor/github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1/tenant.proto';
client.load([], protoPath);

export const options = {
  scenarios: {
    same_tenant_burst: {
      executor: 'constant-vus',
      vus: __ENV.VUS ? parseInt(__ENV.VUS) : 120, // 100+
      duration: __ENV.DURATION || '30s',
      gracefulStop: '5s',
    },
  },
  thresholds: {
    'grpc_req_duration': ['p(95)<1000'], // 95th percentile should be under 1 second
    'checks': ['rate>0.98'], // 98% of checks should pass
    // Note: grpc_errors metrics are not available in k6 v0.47+
    // Use checks to validate responses instead
  },
};

const target = __ENV.TARGET || 'localhost:9092'; // Registry service port

// Fixed tenant to trigger concurrency conflicts
const TENANT_ID = __ENV.TENANT_ID || 'race-acme-fixed-001';
const TENANT_NAME = __ENV.TENANT_NAME || 'Acme Corp Fixed';
const REGION = __ENV.REGION || 'eu10';
const PLAN = __ENV.PLAN || 'standard';

export default function () {
  if (!client.connected) client.connect(target, { plaintext: true });

  const req = {
    id: TENANT_ID,
    name: TENANT_NAME,
    region: REGION,
    role: 1, // ROLE_LIVE = 1
    owner_id: 'load-test-owner',
    owner_type: 'load-test',
    // add other fields your RPC requires
  };

  // Using the correct service path from tenant.proto
  const res = client.invoke('kms.api.cmk.registry.tenant.v1.Service/RegisterTenant', req);

  const ok = check(res, {
    'got gRPC reply': (r) => r && r.status !== undefined,
    // Accept OK and ALREADY_EXISTS as "expected" outcomes
    'ok or already exists': (r) => [grpc.StatusOK, grpc.StatusAlreadyExists].includes(r.status),
    // Reject INTERNAL and UNKNOWN errors
    'no internal errors': (r) => r.status !== grpc.StatusInternal,
    'no unknown errors': (r) => r.status !== grpc.StatusUnknown,
  });

  // Optional: brief pacing so all VUs don’t align perfectly
  sleep(0.1);
}

export function teardown() {
  client.close();
}