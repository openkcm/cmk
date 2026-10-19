# OpenKCM CMK Helm chart

This chart deploys [OpenKCM CMK](https://openkcm.io/components/cmk/cmk-service), the
Customer Managed Keys service: its REST API, the background processes and the
database migrations.

Install it once per **CMK region**, as part of that region's deployment with
ExtAuthZ, Session Manager and the CMK UI. The Registry runs once, in the central
deployment, and sends each tenant of this region to this release; see
[Deployment planning](https://openkcm.io/components/cmk/deployment).

Every value is documented in [values.yaml](values.yaml). Every setting of CMK itself,
under `config`, is described in the
[CMK configuration reference](https://openkcm.io/components/cmk/security/cmk-config);
the [plugin configuration](https://openkcm.io/components/cmk/security/cmk-plugins)
has examples for every plugin. The
[configuration guide](https://openkcm.io/components/cmk/security/configuration) shows
the complete setup with Envoy Gateway, ExtAuthZ, Session Manager and the Registry.

## What the chart creates

CMK is one binary, `cmkd`, that runs one process per subcommand. Each process is a
Deployment with its own ConfigMap and ServiceAccount. All use the same image; each
ConfigMap holds the sections of `config` that its process reads.

| Component | Values | Deployment | ConfigMap | Purpose |
| --- | --- | --- | --- | --- |
| API server | `apiServer` | `<release>-cmk` | `<release>-cmk-config` | The CMK REST API (`cmkd api-server`), with a Service on port 8081 |
| Tenant manager | `tenantManager` | `<release>-cmk-tenant-manager` | `<release>-cmk-tenant-manager-config` | Tenant jobs from the Registry over AMQP |
| Event reconciler | `eventReconciler` | `<release>-cmk-event-reconciler` | `<release>-cmk-event-reconciler-config` | Key and system tasks to the regions over AMQP |
| Task scheduler | `taskScheduler` | `<release>-cmk-task-scheduler` | `<release>-cmk-task-scheduler-config` | Schedules the periodic tasks. Always one replica |
| Task worker | `taskWorker` | `<release>-cmk-task-worker` | `<release>-cmk-task-worker-config` | Runs the scheduled and asynchronous tasks |
| cmkctl | `cmkctl` | `<release>-cmk-cmkctl` | `<release>-cmk-cmkctl-config` | Off by default. An idle pod for the `cmkctl` CLI |
| Migrations | `dbMigrator` | Jobs `<release>-cmk-schema-db-migrator`, `<release>-cmk-data-db-migrator` | `<release>-cmk-db-migrator-config` (hook) | Helm hooks: schema before, data after every install and upgrade |

For a release named `cmk`, the names start with `cmk` instead of `cmk-cmk`.

Optional: NetworkPolicies (`networkPolicy.enabled`), HorizontalPodAutoscalers
(`autoscaling`, `tenantManager.autoscaling`) and PodDisruptionBudgets for the API server
and the tenant manager (`podDisruptionBudget.enabled`).

The chart contains one template file per resource and component, under
`templates/<component>/`; each ConfigMap writes its `config.yaml` section by section.
Shared helpers only build names and labels.

## Prerequisites

- Kubernetes 1.23 or later (`autoscaling/v2`), and Helm 3 or later.
- PostgreSQL. CMK creates one schema per tenant; the user needs the `CREATE`
  privilege.
- Valkey or Redis for the task queue.
- An AMQP 1.0 broker, such as RabbitMQ, for tenant jobs from the Registry and for
  the tasks to the system regions.
- The central [Registry](https://openkcm.io/components/cmk/registry), with a target
  for this CMK region whose queues match `config.tenantManager.amqp`, and this
  region's [Session Manager](https://openkcm.io/components/cmk/session-manager),
  for the tenant manager.
- ExtAuthZ's public client data keys, and Envoy Gateway in front of the API.
- The image `cmk/images/cmk-full-plugins` (default) or `cmk/images/cmk-full-base`.
  The `cmk/images/cmkd` image has no migrations.

## Install

```yaml
# cmk-values.yaml
# image.tag defaults to the chart's appVersion; set it only to deploy another version.

replicaCount: 2

podDisruptionBudget:
  enabled: true

networkPolicy:
  enabled: true
  http:
    from:
      - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: envoy-gateway-system } }

extraVolumes:
  - name: credentials
    projected:
      sources:
        - secret: { name: cmk-database, items: [{ key: password, path: database/password }, { key: ca.crt, path: database/ca.crt }] }
        - secret: { name: cmk-valkey, items: [{ key: password, path: valkey/password }, { key: tls.crt, path: valkey/tls.crt }, { key: tls.key, path: valkey/tls.key }, { key: ca.crt, path: valkey/ca.crt }] }
        - secret: { name: cmk-mtls, items: [{ key: tls.crt, path: mtls/tls.crt }, { key: tls.key, path: mtls/tls.key }, { key: ca.crt, path: mtls/ca.crt }] }
        - secret: { name: cmk-scim, items: [{ key: tls.crt, path: scim/tls.crt }, { key: tls.key, path: scim/tls.key }] }
  - name: client-data-public-keys
    secret: { secretName: cmk-client-data-public-keys }   # ExtAuthZ's public keys only
extraVolumeMounts:
  - { name: credentials, mountPath: /etc/credentials, readOnly: true }
  - { name: client-data-public-keys, mountPath: /etc/signing-keys, readOnly: true }

config:
  database:
    host: { value: postgresql.services.svc.cluster.local }
    user: { value: cmk }
    secret: { source: file, file: { path: /etc/credentials/database/password, format: binary } }
    parameters:
      ssl:
        rootCert: /etc/credentials/database/ca.crt
  scheduler:
    taskQueue:
      host: { value: valkey-primary.storage.svc.cluster.local }
      acl:
        enabled: true
        username: { value: default }
        password: { source: file, file: { path: /etc/credentials/valkey/password, format: binary } }
      secretRef:
        type: mtls
        mtls:
          cert:     { source: file, file: { path: /etc/credentials/valkey/tls.crt, format: binary } }
          certKey:  { source: file, file: { path: /etc/credentials/valkey/tls.key, format: binary } }
          serverCa: { source: file, file: { path: /etc/credentials/valkey/ca.crt, format: binary } }
  services:
    registry:
      enabled: true
      address: registry.services.svc.cluster.local:9092
      secretRef:
        type: mtls
        mtls:
          cert:     { source: file, file: { path: /etc/credentials/mtls/tls.crt, format: binary } }
          certKey:  { source: file, file: { path: /etc/credentials/mtls/tls.key, format: binary } }
          serverCa: { source: file, file: { path: /etc/credentials/mtls/ca.crt, format: binary } }
    sessionManager:
      enabled: true
      address: session-manager.services.svc.cluster.local:9091
      secretRef:
        type: mtls
        mtls:
          cert:     { source: file, file: { path: /etc/credentials/mtls/tls.crt, format: binary } }
          certKey:  { source: file, file: { path: /etc/credentials/mtls/tls.key, format: binary } }
          serverCa: { source: file, file: { path: /etc/credentials/mtls/ca.crt, format: binary } }
  tenantManager:
    secretRef:
      type: mtls
      mtls:
        cert:     { source: file, file: { path: /etc/credentials/mtls/tls.crt, format: binary } }
        certKey:  { source: file, file: { path: /etc/credentials/mtls/tls.key, format: binary } }
        serverCa: { source: file, file: { path: /etc/credentials/mtls/ca.crt, format: binary } }
    amqp:
      url: amqps://rabbitmq.services.svc.cluster.local:5671
      source: /queues/cmk.tenants.requests
      target: /exchanges/cmk/mesh.tenants
  eventProcessor:
    secretRef:
      type: mtls
      mtls:
        cert:     { source: file, file: { path: /etc/credentials/mtls/tls.crt, format: binary } }
        certKey:  { source: file, file: { path: /etc/credentials/mtls/tls.key, format: binary } }
        serverCa: { source: file, file: { path: /etc/credentials/mtls/ca.crt, format: binary } }
    targets:
      - region: eu10
        amqp:
          url: amqps://rabbitmq.services.svc.cluster.local:5671
          target: /exchanges/cmk/eu10.requests
          source: /queues/cmk.eu10.replies
  plugins:
    - name: AWS
      type: KeystoreInstanceKeyOperation
      path: /etc/plugins/keystore/keystoreop-aws
      tags: [default_keystore, hyok]
      logLevel: info
      yamlConfiguration: |
        auth:
          method: perRequest
    - name: scim
      type: IdentityManagementService
      path: /etc/plugins/identity-management/scim
      yamlConfiguration: |
        host: { source: embedded, value: "https://scim.example.com/scim/v2" }
        auth:
          type: mtls
          mtls:
            cert: { source: file, file: { path: /etc/credentials/scim/tls.crt } }
            certKey: { source: file, file: { path: /etc/credentials/scim/tls.key } }
        params:
          groupAttribute: { source: embedded, value: displayName }
          userAttribute: { source: embedded, value: groups.display }
          groupMembersAttribute: { source: embedded, value: "" }
          listMethod: { source: embedded, value: POST }
        authContext:
          source: embedded
          value: |
            hostField: issuer
            basePath: scim/v2
            headerFields: { x-client-id: client_id }
    - { name: noop, type: NotificationService }
    - { name: noop, type: SystemInformationService }
  audit:
    endpoint: http://otel-collector.observability.svc:4318/v1/logs
```

```sh
helm upgrade --install cmk ./charts/cmk -n cmk -f cmk-values.yaml --timeout 15m
```

The release notes list the settings that are missing or not production-ready. Raise
`--timeout` with many tenants: the migration jobs migrate every tenant schema within
the install or upgrade.

## Important values

| Value | Default | Description |
| --- | --- | --- |
| `image.tag` / `image.digest` | chart's `appVersion` | The CMK version to deploy. Leave both empty to deploy the version this chart release is made for |
| `image.repository` | `cmk/images/cmk-full-plugins` | Under `ghcr.io/openkcm` |
| `image.cmkdPath`, `image.cmkctlPath` | `/cmkd`, `/cmkctl` | Binaries the chart starts; the images have no entrypoint |
| `<component>.deploy` | all but `cmkctl` | Which processes to run |
| `replicaCount` | `1` | API server replicas; use two or more in production |
| `<component>.replicaCount` | `1` | Tenant manager, event reconciler and task worker |
| `networkPolicy.enabled` | `false` | **Recommended.** Allow only Envoy Gateway to call the API |
| `config.database`, `config.scheduler.taskQueue` | hosts and credentials empty | **Required** |
| `config.plugins` | `[]` | **Required.** Keystore, identity management, notification and system information plugins |
| `config.clientData.signingKeysPath` | `/etc/signing-keys` | **Required.** Mount ExtAuthZ's public keys here |
| `config.services`, `config.tenantManager` | disabled, empty | **Required** by the tenant manager |
| `config.eventProcessor.targets` | `[]` | One AMQP target per region |
| `config.provisioning.initKeystoreConfig.enabled` | `false` | Adds a keystore to the pool at every API server start. Enable it for the first start only, see [Configuration](#configuration) |
| `extraVolumes`, `extraVolumeMounts`, `extraEnvs` | `[]` | Applied to all components and the migration jobs |

## Security defaults

| Setting | Default |
| --- | --- |
| Pods | Non-root (user, group and `fsGroup` 65532), `RuntimeDefault` seccomp profile, no ServiceAccount token, no Service environment variables |
| Containers | No privilege escalation, all capabilities dropped, read-only root filesystem, writable `/tmp`. `extraInitContainers` and `extraContainers` get only the pod settings: set their own `securityContext` |
| Secrets | No default values. The release notes warn when `config.database.secret` or `config.scheduler.taskQueue.acl.password` is embedded; use `source: file` for every password, key and token |
| Database TLS | `verify-full` |
| Log level | `info`. At `debug`, CMK logs its complete configuration, including embedded secrets |
| Swagger UI and profiling endpoints | Off |

The API server trusts the client data that ExtAuthZ adds to each request. Only
Envoy Gateway may call it: keep the Service type `ClusterIP` and enable
`networkPolicy`. Plugin processes inherit the environment of the CMK containers; pass
plugin credentials through files.

## Configuration

Everything under `config` in `values.yaml` is CMK's own configuration. Every key is listed
there with its default, its allowed values and an example, and each section names the
processes that read it.

Each component's ConfigMap (`templates/<component>/configmap.yaml`) lists the sections its
process reads, and the helper `cmk.config` (`templates/_helpers.tpl`) writes them into
`config.yaml`, mounted at `/etc/cmk/config.yaml` (the tenant
manager: `/etc/tenant-manager/config.yaml`). Only the file is mounted, so
`/etc/cmk/migrations` of the image stays visible.

| Process | Sections |
| --- | --- |
| API server | all except `tenantManager` and `eventProcessor` |
| Tenant manager | `featureGates`, `featureFlags`, `logger`, `status`, `telemetry`, `database`, `databaseReplicas`, `services`, `tenantManager`, `plugins`, `audit`, `certificates`, `cryptoLayer`, `landscape`, `keystorePool`, `configurableContext`, `tenant`, `keys`, `workflow` |
| Event reconciler | `logger`, `status`, `telemetry`, `audit`, `plugins`, `services`, `database`, `databaseReplicas`, `eventProcessor`, `landscape`, `cryptoLayer`, `featureGates`, `featureFlags`, `certificates`, `keystorePool`, `tenant`, `workflow` |
| Task scheduler | `status`, `logger`, `telemetry`, `database`, `scheduler`, `certificates` |
| Task worker | `featureGates`, `featureFlags`, `logger`, `status`, `telemetry`, `database`, `databaseReplicas`, `scheduler`, `configurableContext`, `certificates`, `services`, `plugins`, `audit`, `keystorePool`, `keys`, `landscape`, `cryptoLayer`, `tenant`, `workflow` |
| cmkctl | `featureGates`, `featureFlags`, `logger`, `status`, `telemetry`, `database`, `databaseReplicas`, `scheduler`, `plugins`, `services`, `audit`, `certificates`, `cryptoLayer`, `landscape`, `keystorePool`, `configurableContext`, `tenant`, `keys`, `workflow` |
| db-migrator | `status`, `logger`, `telemetry`, `database`, `databaseReplicas`, `cryptoLayer` |

- `config.environment` and `config.labels` become `application.environment` and
  `application.labels`; `config.http.port` becomes `http.address: ":<port>"`.
  `config.isImmutable` sets the ConfigMaps' `immutable` field and is not written.
- Keys that are not in `values.yaml` are written too, inside the sections above. CMK
  rejects unknown keys at startup, so a misspelled key stops the pods.
- Keep `config.isImmutable: false`. The ConfigMap names do not change with their
  content, so with `true` every upgrade that changes the configuration fails.
- The `checksum/config` annotation restarts a component's pods when its ConfigMap
  changes.
- Secrets that you manage yourself need a restart after a change, for example
  `kubectl rollout restart deployment -l app.kubernetes.io/part-of=cmk`.
- `config.provisioning.initKeystoreConfig` adds its keystore to the pool every time an
  API server pod starts, under a new ID: each replica, restart, rollout and autoscaler
  scale-up. Enable it only for the first start: install with `enabled: true` and one API
  server replica, wait until the API server has started, then upgrade with
  `enabled: false`. The release notes warn while it is enabled.

## Upgrading from 0.1.x

| Change | Action |
| --- | --- |
| The default image is `ghcr.io/openkcm/cmk/images/cmk-full-plugins` (was `cmk-api-server-dev:latest`), with the tag from the chart's `appVersion` | None; set `image.registry`, `image.repository` and `image.tag` if you use another image |
| The chart starts `/cmkd <subcommand>` (`/cmkctl sleep`) by default | Remove `<component>.image.command` and `args` from your values, or keep them; they still take precedence |
| The task scheduler and task worker are deployed by default | Configure `config.scheduler.taskQueue` and the plugins, or set `deploy: false` |
| The event reconciler follows `eventReconciler.deploy` (was `apiServer.deploy`) and gets its ServiceAccount | None |
| Each component gets every section its process reads. Before, several processes ran with CMK's defaults for sections they read, for example `tenant`, `workflow`, `keys`, `cryptoLayer`, `featureFlags` and `audit`, and the data migration had no `cryptoLayer` | Check these sections in your values |
| `config.workflow`, `audit`, `telemetry`, `provisioning` and `databaseReplicas` are set to CMK's defaults in `values.yaml` | None |
| No example credentials or hosts; `cryptoLayer.certX509Trusts` is an empty list; `database.parameters.ssl.mode` is `verify-full` | Set your own values |
| `config.telemetry` is not set (logs and traces were enabled with a placeholder endpoint) | Configure telemetry if you need it |
| `config.isImmutable` defaults to `false` (was `true`), and the ConfigMaps are named after the release: `<release>-cmk-config`, `<release>-cmk-task-worker-config`, … (were `cmk-config`, `cmk-task-worker-config`, … in every release) | Release named `cmk`: the names stay the same, and Kubernetes rejects changes to the old immutable ConfigMaps. Delete them once before the upgrade, see below. Other releases: none; Helm creates the new ConfigMaps and deletes the old ones |
| Migration jobs and their ConfigMap are named after the release | None for a release named `cmk` |
| `config.aws` was removed from `values.yaml`; it was never written into the configuration | None |
| `config.yaml` is mounted as a file (`subPath`) | None |
| New security defaults (user, group and `fsGroup` 65532, read-only root filesystem), probe thresholds, resource requests and limits | Override `podSecurityContext`, `securityContext`, the probes and `resources` if needed |
| `serviceAccount.automount` defaults to `false`; with `create: false` all components use `serviceAccount.name` (was `<name>-<component>`) | Create your ServiceAccount, or keep `create: true` |
| `autoscaling` applies to the API server only; the tenant manager has `tenantManager.autoscaling` | Move the settings |
| The task scheduler always runs one replica with the `Recreate` strategy | None |
| PodDisruptionBudgets without `minAvailable` or `maxUnavailable` use `maxUnavailable: 1` (was `minAvailable: 1`) | Set `minAvailable` explicitly if you want the old behaviour |
| The migration jobs get no `extraContainers`, but `dbMigrator.extraContainers`, resources and retry settings | Move containers that exit on their own to `dbMigrator.extraContainers` |
| `featureFlagsConfigMap` must be set when `config.featureFlags.enabled` is true | Set it; the chart fails otherwise |
| The values for local development are no longer part of the chart package: `values-dev.yaml` stays in the cmk repository next to the chart (`charts/cmk/values-dev.yaml`), for `make start-cmk` only | Copy settings from `values.yaml` if you used them as a template |

For a release named `cmk`, delete the immutable ConfigMaps of 0.1.x right before the
upgrade. Running pods keep their mounted `config.yaml`; the upgrade creates the
ConfigMaps again and restarts the pods.

```sh
kubectl delete configmap -n <namespace> --ignore-not-found \
  cmk-config cmk-tenant-manager-config cmk-event-reconciler-config \
  cmk-task-scheduler-config cmk-task-worker-config cmk-cmkctl-config
```

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| Pods stay in `ContainerCreating` | A Secret or ConfigMap from `extraVolumes` is missing |
| Pod exits with `has invalid keys` | A key under `config` is not a CMK setting |
| Upgrade fails with `field is immutable` on a ConfigMap | Immutable ConfigMaps from 0.1.x or `config.isImmutable: true`: delete the ConfigMaps, see [Upgrading from 0.1.x](#upgrading-from-01x), and keep `config.isImmutable: false` |
| API server exits with `creating data loader: path does not exist` | Nothing is mounted at `config.clientData.signingKeysPath` |
| Tenant manager exits with `only insecure or mtls secrets are supported` or `AMQP URL must be specified` | `config.tenantManager` is missing or incomplete |
| Task worker exits with `failed to load notification plugin` | Add a `NotificationService` and a `SystemInformationService` plugin |
| Install or upgrade times out in the schema migration | Many tenants: raise `--timeout`; check `kubectl logs job/<release>-cmk-schema-db-migrator` |
| Tenant schemas are empty | Wrong `config.database.migrator` paths, or an image without migrations |

The [configuration reference](https://openkcm.io/components/cmk/security/cmk-config#troubleshooting)
lists more symptoms.

## Development

The unit tests of this chart are in `helm-tests/unit` of the cmk repository. They render
the chart with `helm template` and load each process's `config.yaml` with CMK's own
configuration loader. Run them from the repository root; they need `helm` in `PATH`:

```sh
make helm-unit-test
```

`make start-cmk` runs CMK locally in k3d with this chart and `charts/cmk/values-dev.yaml`
(the development image `cmk-api-server-dev` and development credentials).
`make k3d-add-cmk` rebuilds the image and redeploys the chart; `make tenant-cli ARGS="…"`
and `make task-cli ARGS="…"` run `cmkctl tenant` and `cmkctl task` in the cmkctl pod.
`values-dev.yaml` is excluded from the chart package (`.helmignore`).

## Uninstall

```sh
helm uninstall cmk -n cmk
```

The database, the task queue and the Secrets you created yourself are not removed.
Helm does not remove hook resources either: a migration job or its ConfigMap left by a
failed migration stays until the next install or upgrade replaces it.
