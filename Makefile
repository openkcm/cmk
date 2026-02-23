.PHONY: default test coverage clean install-gotestsum squash codegen docker-compose clean-docker-compose swagger-ui \
swagger-ui-stop go-imports install-gci install-golines install-goimports lint cmk-env docker-dev-build tidy \
prepare_integration_test clean_integration_test build_test_plugins clean_plugins clean_test benchmark

CMK_API_V1_SPEC_PATH := apis/cmk
CMK_API_V1_OUT_PATH := internal/api/cmkapi
SWAGGER_UI_HOST_PORT := 8087
CMK_APP_NAME := cmk-api-server
CMK_DEV_TARGET := dev
CMK_TEST_TARGET := test
pwd=$(shell pwd)
GIT_CREDENTIAL_HELPER=$(shell git config credential.helper)
GIT_USERNAME=$(shell echo "" | git credential-$(GIT_CREDENTIAL_HELPER) get | awk -F= '$$1=="username"{print $$2}')
GIT_PASSWORD=$(shell echo "" | git credential-$(GIT_CREDENTIAL_HELPER) get | awk -F= '$$1=="password"{print $$2}')
SIS_PLUGIN ?= "uli"
ACTIVE_PLUGINS := "{hyok,default_keystore,keystore_provider,$(SIS_PLUGIN),cert_issuer,identity_management,notification}"

# get git values
squash: HEAD := $(shell git rev-parse HEAD)
squash: CURRENT_BRANCH := $(shell git branch --show-current)
squash: MERGE_BASE := $(shell git merge-base origin/main $(CURRENT_BRANCH))

default: test

.PHONY: run
run:
	AWS_ACCESS_KEY_ID="exampleAccessKeyID" AWS_SECRET_ACCESS_KEY="exampleSecretAccessKey" go run ./cmd/api-server

test: install-gotestsum build_test_plugins
	rm -rf cover cover.* junit.xml
	mkdir -p cover
	go clean -testcache

	# Run tests with coverage
	env TEST_ENV=make gotestsum --rerun-fails --format testname --junitfile junit.xml \
		--packages="./internal/... ./utils... ./cmd/... ./test/security/... ./test/db-migration/..." \
		-- -count=1 -covermode=atomic -coverpkg=./... \
		-args -test.gocoverdir=$(pwd)/cover

	go tool covdata textfmt -i=./cover -o cover.out

	$(MAKE) clean_test_plugins


benchmark:
	go test ./benchmark -bench=.


clean_test:
	$(MAKE) clean-postgres-db
	$(MAKE) clean-rabbitmq
	$(MAKE) clean_test_plugins


integration_test:  prepare_integration_test
	gotestsum --format testname ./test/integration/...
	$(MAKE) clean_integration_test

prepare_integration_test: clean_integration_test install-gotestsum spin_sysinfo_mock \
	build_sysinfo_plugin build_certissuer_plugin build_notification_plugin wait_for_sysinfo_mock

clean_integration_test:
	$(MAKE) clean_sysinfo_plugin
	$(MAKE) clean_certissuer_plugin
	$(MAKE) clean_identitymanagement_plugin
	$(MAKE) clean_notification_plugin
	$(MAKE) clean_sysinfo_mock

build_sysinfo_plugin:
	$(MAKE) -C sis-plugins

clean_sysinfo_plugin:
	$(MAKE) -C sis-plugins clean

build_certissuer_plugin:
	$(MAKE) -C cert-issuer-plugins

clean_certissuer_plugin:
	$(MAKE) -C cert-issuer-plugins clean

build_identitymanagement_plugin:
	$(MAKE) -C identity-management-plugins

clean_identitymanagement_plugin:
	$(MAKE) -C identity-management-plugins clean

build_notification_plugin:
	$(MAKE) -C notification-plugins

clean_notification_plugin:
	$(MAKE) -C notification-plugins clean

# Run tests with coverage
coverage: test
	go tool cover -html=cover.out

# installs gotestsum test helper
install-gotestsum:
	(cd /tmp && go install gotest.tools/gotestsum@latest)

clean:
	rm -fR bin
	rm -f cover.* junit.xml *.out

# squash will take all commits on the current branch (commits done after branched away from main) and squash them into a single commit on top of main HEAD.
# The commit message of this single commit is compiled from all previous commits. Please modify as needed.
# After all: force push to origin.
squash:
	@git diff --quiet || (echo "you have untracked changes, stopping" && exit 1)
	@echo "*********** if anything goes wrong, you can simply reset all the changes made executing: git reset --hard $(HEAD)"
	git log --pretty=format:"%+* %B" --reverse $(MERGE_BASE).. > git_log.tmp
	git branch safe/$(CURRENT_BRANCH)
	git reset $(MERGE_BASE)
	git stash
	git reset --hard origin/main
	git stash apply
	git add .
	git commit -F git_log.tmp
	@rm -f git_log.tmp


#
# Open API Code Generation
#

# codegen will generate desired API clients, see below help text for supported apis
codegen: install-codegen
ifeq ($(api),)
	@echo "API is not defined. Please provide name of api to generate. Parameters allowed:"
	@echo "		- cmk"
	@echo "		- sysinfo"
	@echo "		- all"
	@echo "CMK API Example: make codegen api=cmk"
	exit 1
endif

# Generate CMK API v1
ifeq ($(api), $(filter $(api),all cmk))
	@echo "Generating CMK API from Swagger definition"
	@oapi-codegen -config "$(CMK_API_V1_SPEC_PATH)/.config.yaml" "$(CMK_API_V1_SPEC_PATH)/cmk-ui.yaml"
endif

install-codegen:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1

.PHONY: codegen-commit
codegen-commit:
	git diff --cached --exit-code > /dev/null  # Fail if there is currently anything staged
	@echo "Adding generated files to git"
	git add $(CMK_API_V1_OUT_PATH)/cmkapi.go $(CMK_API_V1_OUT_PATH)/patch_cmkapi.go
	! git diff --cached --exit-code > /dev/null  # Fail if there is nothing staged
	@echo "Committing"
	git commit -m "Update generated CMK api code"



TEST_PLUGINS_DIR := ./internal/testutils/testplugins
TEST_PLUGINS := $(shell find $(TEST_PLUGINS_DIR) -mindepth 1 -maxdepth 1 -type d)
PLUGIN_NAME := testpluginbinary

build_test_plugins:
	@echo "Building plugins..."
	@for plugin_dir in $(TEST_PLUGINS); do \
		echo "Building $${plugin_dir}"; \
		(cd $${plugin_dir} && go build -o $(PLUGIN_NAME) .); \
		if [ $$? -ne 0 ]; then \
			echo "Failed to build $${plugin_dir}"; \
			exit 1; \
		fi; \
	done
	@echo "All plugins built successfully"

clean_test_plugins:
	@find $(TEST_PLUGINS_DIR) -name "$(PLUGIN_NAME)" -exec rm -f {} +
	@echo "Cleaned all plugin binaries"
	# Kill all leftover processes
	@killall testpluginbinary || true


stack_name := cmk-stack-local
test-db-container := postgres-test
docker_compose_file := local_env/docker-compose.yml

# Start docker-compose stack
docker-compose:
	docker-compose -f $(docker_compose_file) -p $(stack_name) up -d
	docker-compose -f $(docker_compose_file) logs --tail 10 -f

# Stop and remove docker-compose stack
clean-docker-compose:
	docker-compose -f $(docker_compose_file) -p $(stack_name) down -v
	docker-compose -f $(docker_compose_file) -p $(stack_name) rm -f -v


# Start Swagger UI
.SILENT: swagger-ui
swagger-ui: swagger-ui-stop
	@echo Starting Swagger UI server ...
	docker pull swaggerapi/swagger-ui
	docker run --detach --name swagger-ui -p $(SWAGGER_UI_HOST_PORT):8080 \
	  -e BASE_URL=/swagger -e API_URL="$(CMK_API_V1_SPEC_PATH)/cmk-ui.yaml" \
	  -v $(pwd)/$(CMK_API_V1_SPEC_PATH):/usr/share/nginx/html/$(CMK_API_V1_SPEC_PATH) \
	  swaggerapi/swagger-ui 1>/dev/null && \
	  echo Access Swagger UI at \'http:/\/\localhost:$(SWAGGER_UI_HOST_PORT)/swagger\'

.SILENT: swagger-ui-stop
swagger-ui-stop:
	@echo Stopping Swagger UI server \(if exists\) ...
	docker stop swagger-ui >/dev/null 2>&1 || true
	docker rm swagger-ui >/dev/null 2>&1 || true

#
# Formatting and Linting
#

go-imports: install-goimports install-golines install-gci
	find . -name "*.go" -exec sh -c \
      'gofmt -w "$$1" && \
      goimports -w "$$1" && \
      golines -w "$$1" && \
      gci write --skip-generated -s standard -s default -s "prefix(github.com/openkcm/cmk)" \
      -s blank -s dot -s alias -s localmodule "$$1"' sh {} \;

go-imports-changed:
	@tempfile=$$(mktemp); \
	git --no-pager diff --name-only HEAD | grep '\.go$$' > $$tempfile; \
	while read file; do \
	  gofmt -w "$$file" && \
	  goimports -w "$$file" && \
	  golines -w "$$file" && \
	  gci write --skip-generated -s standard -s default -s "prefix(github.com/openkcm/cmk)" \
	  -s blank -s dot -s alias -s localmodule "$$file"; \
	done < $$tempfile; \
	rm -f $$tempfile

install-gci:
	command -v gci >/dev/null 2>&1 || go install github.com/daixiang0/gci@latest

install-golines:
	command -v golines >/dev/null 2>&1 || go install github.com/segmentio/golines@latest

install-goimports:
	command -v goimports >/dev/null 2>&1 || go install golang.org/x/tools/cmd/goimports@latest

lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0
	golangci-lint run -v --fix

cmk-env:
	cp .env.dist .env

TAG := latest
CMK_IMAGE_NAME := $(CMK_APP_NAME)-$(CMK_DEV_TARGET):$(TAG)
DOCKERFILE_DIR ?= .
DOCKERFILE_NAME := Dockerfile.dev
CONTEXT_DIR := .

# Target to build Docker image
docker-dev-build:
	@GIT_LOGIN=$(GIT_USERNAME) GIT_PASS=$(GIT_PASSWORD) docker build --secret type=env,id=git-login,env=GIT_LOGIN --secret type=env,id=git-password,env=GIT_PASS -f $(DOCKERFILE_DIR)/$(DOCKERFILE_NAME) -t $(CMK_IMAGE_NAME) $(CONTEXT_DIR)

# Mock CLD server
spin_sysinfo_mock:
	$(MAKE) -C sis-plugins/mocks/cld

wait_for_sysinfo_mock:
	$(MAKE) -C sis-plugins/ wait_for_mock

# Stop and remove SystemInfo mock server
clean_sysinfo_mock:
	$(MAKE) -C sis-plugins/mocks/cld cleanCLDServer

test-psql-replica:
	env TEST_ENV=make gotestsum --format testname ./test/integration/psqlreplicatests

test-async:
	env TEST_ENV=make gotestsum --format testname ./test/integration/async_test/

# Signing keys
# Need for testing signing functionality of client data in HTTP requests headers
# The private key is unencrypted for simplicity
generate-signing-keys:
	@echo "Generating RSA signing key pairs (if not already present)..."
	@mkdir -p env/blueprints/signing-keys
	@if [ ! -f env/blueprints/signing-keys/private_key01.pem ]; then \
		echo "Generating private key (unencrypted)..."; \
		openssl genpkey -algorithm RSA -out env/blueprints/signing-keys/private_key01.pem; \
	else \
		echo "Private key already exists, skipping generation."; \
	fi
	@if [ ! -f env/blueprints/signing-keys/key01.pem ]; then \
		echo "Extracting public key..."; \
		openssl rsa -pubout -in env/blueprints/signing-keys/private_key01.pem -out env/blueprints/signing-keys/key01.pem; \
	else \
		echo "Public key already exists, skipping generation."; \
	fi
	@echo "Signing key pairs ready in env/blueprints/signing-keys/"
clean-signing-keys:
	@echo "Cleaning signing key pairs..."
	@rm -f env/blueprints/signing-keys/private_key01.pem
	@rm -f env/blueprints/signing-keys/key01.pem
	@echo "Signing key pairs cleaned."


##K3D

.PHONY: install-k3d start-k3d k3d-build-image k3d-build-otel-image k3d-build-audit-server-image \
k3d-build-producer-% k3d-deploy-% clean-k3d delete-cluster psql-add-to-cluster redis-add-to-cluster helm-install-rabbitmq

PSQL_RELEASE_NAME := cmk-postgresql
REDIS_RELEASE_NAME := cmk-redis
REDIS_USER := redis
CMK_USERNAME := postgres
CMK_PASS := secret
CMK_DB := cmk
CMK_DB_ADMIN_PASS_KEY := secretKey

PLUGIN_SECRETS ?= \
    signing-keys:env/secret/signing-keys \
    scim:env/secret/identity-management/scim.json \
	event-processor-credentials:env/secret/event-processor

create-empty-secrets:
	# This Error code 1 on makefile but it seems to work
	- mkdir -p env/secret
	- cp -nr env/blueprints/. env/secret/.

create-plugin-secret:
	@echo "Creating plugin secrets..."
	@for secret_config in $(PLUGIN_SECRETS); do \
		secret_name=$$(echo $$secret_config | cut -d':' -f1); \
		secret_path=$$(echo $$secret_config | cut -d':' -f2); \
		echo "Creating secret: $$secret_name from $$secret_path"; \
		kubectl create secret generic $$secret_name \
			--namespace $(NAMESPACE) \
			--from-file="$$secret_path" || true; \
	done
	@echo "All plugin secrets created successfully"

psql-add-to-cluster:
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update
	helm upgrade --install '$(PSQL_RELEASE_NAME)' bitnami/postgresql \
	  --set image.repository=bitnamilegacy/postgresql \
	  --set global.postgresql.auth.username=$(CMK_USERNAME) \
	  --set global.postgresql.auth.password=$(CMK_PASS) \
	  --set global.postgresql.auth.database=$(CMK_DB) \
	  --set global.postgresql.auth.secretKeys.adminPasswordKey=$(CMK_DB_ADMIN_PASS_KEY) \
	  --namespace $(NAMESPACE)

create-registry-db: psql-port-forward
	sleep 2
	PGPASSWORD=$(CMK_PASS) psql -h localhost -p 5432 -U $(CMK_USERNAME) -f ./local_env/registry.sql

psql-port-forward: wait-for-psql
	@if ! lsof -i :5432 >/dev/null; then \
		echo "Start 5432 port-forward"; \
		kubectl port-forward svc/$(PSQL_RELEASE_NAME) 5432:5432 -n $(NAMESPACE) & \
	else \
		echo "Port 5432 already forwarded"; \
	fi

redis-add-to-cluster:
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update
	helm upgrade --install '$(REDIS_RELEASE_NAME)' bitnami/redis \
	  --namespace $(NAMESPACE) \
	  --set image.repository=bitnamilegacy/redis \
	  --set auth.password=$(CMK_PASS) \
	  --set architecture=standalone

KUBECTL_CONFIG=${HOME}/.config/k3d/kubeconfig-$(CLUSTER_NAME).yaml
CLUSTER_NAME=cmkcluster
NAMESPACE=cmk
PATH_TO_INIT_VOLUME=$(pwd)/local_env/local-kms/init

# Target to install k3d using wget
install-k3d:
	@echo "Installing k3d using wget."
	@if ! command -v k3d >/dev/null 2>&1; then \
		wget -q -O - https://raw.githubusercontent.com/rancher/k3d/main/install.sh | bash; \
	else \
		echo "k3d is already installed."; \
	fi

# Checks docker version and if engine is running with colima sets a DNS Fix
# Creates CMK namespace
start-k3d: install-k3d delete-cluster clean-k3d
	@echo "Starting k3d."
	@if docker version | grep -q 'colima'; then \
	   K3D_FIX_DNS=0 k3d cluster create $(CLUSTER_NAME) -p "30083:30083@server:0" --api-port 127.0.0.1:6443; \
	else \
	   k3d cluster create $(CLUSTER_NAME) -p "30083:30083@server:0" --api-port 127.0.0.1:6443; \
	fi
	k3d kubeconfig merge $(CLUSTER_NAME) --kubeconfig-switch-context; \
	kubectl create namespace $(NAMESPACE)

# Target to build Docker imgage within k3d 
k3d-build-image: docker-dev-build
	@echo "Importing docker image into k3d"
	k3d image import $(CMK_IMAGE_NAME) -c $(CLUSTER_NAME)

VALUE_DIR ?= charts/cmk

k3d-apply-helm-chart:
	@echo "Applying Helm chart."
	helm upgrade --install $(CHART_NAME) $(CHART_DIR) --namespace $(APPLY_NAMESPACE) \
		--set volumePath=$(PATH_TO_INIT_VOLUME) \
		--set config.activePlugins=$(ACTIVE_PLUGINS) \
		-f $(VALUE_DIR)/values-dev.yaml

k3d-apply-cmk-helm-chart:
	@echo "Applying CMK Helm chart."
	$(MAKE) k3d-apply-helm-chart CHART_NAME=cmk CHART_DIR=$(pwd)/charts/cmk APPLY_NAMESPACE=$(NAMESPACE)

k3d-build-helm:
	helm dependency build ./charts/cmk

# Target to clean everything in the namespace
clean-k3d:
	@echo "Cleaning everything in the cmk namespace in k3d."
	@if kubectl --kubeconfig=${KUBECTL_CONFIG} get namespace $(NAMESPACE) > /dev/null 2>&1; then \
	   kubectl --kubeconfig=${KUBECTL_CONFIG} delete namespace $(NAMESPACE) --ignore-not-found=true; \
	else \
	   echo "Namespace $(NAMESPACE) does not exist."; \
	fi

# Target to delete the k3d cluster
# There is a bug where the cluster exists but doesn't appear in list sometimes
delete-cluster:
	@echo "Deleting k3d cluster '$(CLUSTER_NAME)'."
	@if k3d cluster list | grep -q '$(CLUSTER_NAME)'; then \
	   k3d cluster delete $(CLUSTER_NAME); \
	else \
	   echo "k3d cluster '$(CLUSTER_NAME)' does not exist."; \
	fi

start-cmk: generate-signing-keys start-k3d create-empty-secrets create-plugin-secret psql-add-to-cluster redis-add-to-cluster helm-install-rabbitmq helm-install-otel-collector k3d-add-cmk

k3d-add-cmk:
	@echo "Building the cmk image within k3d."
	@$(MAKE) k3d-build-image
	@$(MAKE) k3d-rebuild-cmk

k3d-rebuild-cmk: k3d-build-helm k3d-apply-cmk-helm-chart k3d-restart-cmk-pod port-forward

k3d-restart-cmk-pod:
	@echo "Restarting the cmk pod."
	kubectl rollout restart deployment cmk -n $(NAMESPACE)
	sleep 5

wait-for-pod:
	@echo "Waiting for pod with label $(LABEL) in namespace $(NAMESPACE) to be Running..."
	@while [ -z "$$(kubectl get pod -n $(NAMESPACE) -l $(LABEL) -o jsonpath='{.items[*].metadata.name}')" ]; do \
		echo "No pods found, waiting for pod creation..."; \
		sleep 2; \
	done
	@while [ "$$(kubectl get pod -n $(NAMESPACE) -l $(LABEL) -o jsonpath='{.items[0].status.phase}' 2>/dev/null)" != "Running" ]; do \
		echo "Pod not ready, waiting..."; \
		sleep 2; \
	done
	@echo "Pod is Running!"


wait-for-svc-cmk:
	@$(MAKE) wait-for-pod LABEL=app.kubernetes.io/name=cmk

port-forward: wait-for-svc-cmk
	kubectl port-forward --namespace $(NAMESPACE) svc/cmk 8080:8081

apply-kms-local-chart:
	@echo "Applying Helm chart."
	helm upgrade --install kms-local $(CMK_HELM_CHART)/aws-kms --namespace $(NAMESPACE) --set volume.path=$(pwd)/local-env/local-kms/init

wait-for-psql:
	@$(MAKE) wait-for-pod LABEL=app.kubernetes.io/name=postgresql

import-test-data: wait-for-psql wait-for-svc-cmk
	kubectl port-forward svc/$(PSQL_RELEASE_NAME) 5432:5432 -n $(NAMESPACE) &
	sleep 2
	PGPASSWORD=$(CMK_PASS) psql -h localhost -p 5432 -U $(CMK_USERNAME) -d $(CMK_DB) -f ./local_env/data.sql

install-psql-macos:
	@echo "Installing psql"
	@if ! command -v psql >/dev/null 2>&1; then \
		brew update; \
		brew install libpq; \
		brew link --force libpq; \
	else \
		echo "psql is already installed."; \
	fi

tidy:
	go mod tidy

# Install RabbitMQ using Helm
helm-install-rabbitmq:
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update
	# Set your desired username and password
	helm upgrade --install rabbitmq bitnami/rabbitmq \
		--set auth.username=guest,auth.password=guest --namespace $(NAMESPACE) \
		--set image.repository=bitnamilegacy/rabbitmq \
		--set global.security.allowInsecureImages=true

# Port-forward RabbitMQ service
port-forward-rabbitmq:
	kubectl port-forward --namespace $(NAMESPACE) svc/rabbitmq 5672:5672 15672:15672 &

# Uninstall RabbitMQ
helm-uninstall-rabbitmq:
	helm uninstall rabbitmq --namespace $(NAMESPACE)

# Prerequisites:
# - cmk-postgresql must be running
# - RabbitMQ must be running
helm-install-registry: create-registry-db
	helm upgrade --install registry oci://ghcr.io/openkcm/charts/registry \
		--namespace $(NAMESPACE) \
		--set image.tag=v1.5.0 \
		--set config.database.host=cmk-postgresql \
		--set config.database.user.value=$(CMK_USERNAME) \
		--set config.database.password.value=$(CMK_PASS) \
		--set config.orbital.targets[0].region=emea \
		--set config.orbital.targets[0].connection.type=amqp \
		--set config.orbital.targets[0].connection.amqp.url="amqp://rabbitmq.cmk.svc.cluster.local:5672" \
		--set config.orbital.targets[0].connection.amqp.source="cmk.global.tenants" \
		--set config.orbital.targets[0].connection.amqp.target="cmk.emea.tenants" \
		--set config.orbital.targets[0].connection.auth.type=none
	sleep 5
	kubectl port-forward svc/registry 9092:9092 -n $(NAMESPACE) &

helm-uninstall-registry:
	helm uninstall registry -n $(NAMESPACE)

generate-task-proto:
	@echo "Generating task proto file"
	protoc -I . \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		internal/event-processor/proto/*.proto
	goimports -w -local=github.com/openkcm/cmk internal/event-processor/proto/*.pb.go

tenant-cli:
	kubectl exec -it -n cmk deploy/cmk-tenant-manager-cli -- ./bin/tenant-manager-cli $(ARGS)

task-cli:
	kubectl exec -it -n cmk deploy/cmk-task-cli -- ./bin/task-cli $(ARGS)

provision-tenants-k3d:
	$(MAKE) tenant-cli ARGS="create -i tenant1 -r emea -s STATUS_ACTIVE -R ROLE_LIVE"
	$(MAKE) tenant-cli ARGS="create -i tenant2 -r emea -s STATUS_ACTIVE -R ROLE_TEST"

default: test

build-tenant-cli:
	@echo "Building tenant-manager-cli binary..."
	go build -o tm ./cmd/tenant-manager-cli

provision-tenants-locally: build-tenant-cli
	./tm create -i tenant1-id -r eu10 -s STATUS_ACTIVE -R ROLE_LIVE
	./tm create -i tenant2-id -r eu10 -s STATUS_ACTIVE -R ROLE_TEST

helm-install-otel-collector:
	helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
	helm repo update
	helm install otel-collector open-telemetry/opentelemetry-collector \
		--namespace $(NAMESPACE) \
		--create-namespace \
		-f ./local_env/otel-values.yaml

helm-uninstall-otel-collector:
	helm uninstall otel-collector --namespace $(NAMESPACE)