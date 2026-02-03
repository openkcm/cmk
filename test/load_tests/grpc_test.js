import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';

const client = new grpc.Client();
// Load proto from vendor directory (source of truth) or fallback to local copy
// The vendor directory is populated by 'go mod vendor' command
const protoPath = __ENV.PROTO_PATH || '../../vendor/github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1/tenant.proto';
client.load([], protoPath);

const CA_FILE = __ENV.CA_FILE ? open(__ENV.CA_FILE) : undefined;
const CERT_FILE = __ENV.CERT_FILE ? open(__ENV.CERT_FILE) : undefined;
const KEY_FILE = __ENV.KEY_FILE ? open(__ENV.KEY_FILE) : undefined;
const USE_TLS = __ENV.USE_TLS === 'true';
const INSECURE_SKIP_VERIFY = __ENV.INSECURE_SKIP_VERIFY === 'true';
const BROKER = __ENV.BROKER || 'localhost:9092';

const GLOBAL_CERTS = USE_TLS
    ? {
        tls: true,
        tls_ca_cert: CA_FILE,
        tls_cert: CERT_FILE,
        tls_key: KEY_FILE,
        insecure_skip_verify: INSECURE_SKIP_VERIFY,
    }
    : { plaintext: true };

export const options = {
    vus: 30,
    duration: '10s',
};

export function setup() {
    return { broker: BROKER, options: GLOBAL_CERTS };
}

export default function (data) {
    client.connect(data.broker, data.options);

    const payload = {
        name: 'SuccessFactor2',
        id: crypto.randomUUID(),
        region: 'emea',
        ownerId: 'owner123',
        ownerType: 'owner_type',
        role: 'ROLE_LIVE',
    };

    const response = client.invoke(
        'kms.api.cmk.registry.tenant.v1.Service/RegisterTenant',
        payload
    );

    check(response, {
        'status OK': (r) => r && r.status === grpc.StatusOK,
    });

    client.close();
    sleep(1);
}
