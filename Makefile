TF ?= ./bin/tf
COMPONENT ?=
LAYER ?=
OVERLAYS ?=
TF_ARGS ?=

.PHONY: plan apply destroy plan-destroy refresh-kubeconfig refresh-postgres-env refresh-ansible-secrets bootstrap-svartalfheim-storage postgres-refresh postgres ansible-minio-host ansible-minio-state ansible-minecraft-vm ansible-kubernetes-worker

check-vars:
	@if [ -z "$(COMPONENT)" ] || [ -z "$(LAYER)" ]; then \
		echo "Usage: make <plan|apply|destroy|plan-destroy> COMPONENT=<name> LAYER=<name> [OVERLAYS=\"layer1 layer2\"] [TF_ARGS=\"...\"]"; \
		exit 1; \
	fi

plan: check-vars
	$(TF) plan $(COMPONENT) $(LAYER) $(OVERLAYS) -- $(TF_ARGS)

apply: check-vars
	$(TF) apply $(COMPONENT) $(LAYER) $(OVERLAYS) -- $(TF_ARGS)

destroy: check-vars
	$(TF) destroy $(COMPONENT) $(LAYER) $(OVERLAYS) -- $(TF_ARGS)

plan-destroy: check-vars
	$(TF) plan $(COMPONENT) $(LAYER) $(OVERLAYS) -- -destroy $(TF_ARGS)

refresh-kubeconfig:
	@./scripts/refresh-kubeconfig.sh

refresh-postgres-env:
	@set -e; \
	env_file=".devcontainer/.env"; \
	secret_name="app-db-bootstrap"; \
	secret_namespace="data"; \
	gateway_namespace="ingress"; \
	gateway_service="internal-gateway-istio"; \
	pg_host="$$(kubectl -n "$$gateway_namespace" get svc "$$gateway_service" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"; \
	pg_port="5432"; \
	pg_db="app"; \
	if [ -z "$$pg_host" ]; then \
		echo "Error: failed to load postgres gateway IP from $$gateway_namespace/$$gateway_service" >&2; \
		exit 1; \
	fi; \
	pg_user="$$(kubectl -n "$$secret_namespace" get secret "$$secret_name" -o jsonpath='{.data.username}' | base64 -d)"; \
	pg_pass="$$(kubectl -n "$$secret_namespace" get secret "$$secret_name" -o jsonpath='{.data.password}' | base64 -d)"; \
	if [ -z "$$pg_user" ] || [ -z "$$pg_pass" ]; then \
		echo "Error: failed to load postgres credentials from $$secret_namespace/$$secret_name" >&2; \
		exit 1; \
	fi; \
	upsert_env() { \
		key="$$1"; \
		value="$$2"; \
		if grep -q "^$${key}=" "$$env_file"; then \
			sed -i "s|^$${key}=.*|$${key}='$${value}'|" "$$env_file"; \
		else \
			printf "%s='%s'\n" "$$key" "$$value" >> "$$env_file"; \
		fi; \
	}; \
	upsert_env DB_HOST "$$pg_host"; \
	upsert_env DB_PORT "$$pg_port"; \
	upsert_env DB_NAME "$$pg_db"; \
	upsert_env DB_USER "$$pg_user"; \
	upsert_env DB_PASS "$$pg_pass"; \
	echo "Updated $$env_file with Postgres connection details for $$pg_host:$$pg_port/$$pg_db"; \
	echo "Run: . /home/vscode/.env"

refresh-ansible-secrets:
	@./scripts/ansible-fetch-secrets.sh

bootstrap-svartalfheim-storage:
	@./scripts/bootstrap-svartalfheim-storage.sh

postgres:
	@set -e; \
	. /home/vscode/.env; \
	if ! command -v psql >/dev/null 2>&1; then \
		echo "Error: psql is not installed in the devcontainer. Rebuild the devcontainer to pick up postgresql-client." >&2; \
		exit 1; \
	fi; \
	connect() { \
		PGPASSWORD="$$DB_PASS" psql -h "$$DB_HOST" -p "$$DB_PORT" -U "$$DB_USER" -d "$$DB_NAME"; \
	}; \
	if ! connect; then \
		echo "Initial connection failed; refreshing Postgres connection details from the cluster and retrying once." >&2; \
		$(MAKE) refresh-postgres-env; \
		. /home/vscode/.env; \
		connect; \
	fi

ansible-minio-host:
	@./scripts/ansible-run-playbook.sh -i ansible/inventory/hosts.ini ansible/playbooks/minio-external-pi-host.yml

ansible-minio-state:
	@./scripts/ansible-run-playbook.sh -i ansible/inventory/hosts.ini ansible/playbooks/minio-external-pi.yml

ansible-minecraft-vm:
	@ANSIBLE_FETCH_MINIO_SECRETS=0 ./scripts/ansible-run-playbook.sh -i ansible/inventory/hosts.ini ansible/playbooks/minecraft-vm.yml

ansible-kubernetes-worker:
	@set -e; \
	args="-i ansible/inventory/hosts.ini ansible/playbooks/kubernetes-terraform-node.yml"; \
	if [ -n "$(LIMIT)" ]; then \
		args="$$args --limit $(LIMIT)"; \
	fi; \
	ANSIBLE_FETCH_MINIO_SECRETS=0 ./scripts/ansible-run-playbook.sh $$args
