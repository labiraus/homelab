TF ?= ./bin/tf
COMPONENT ?=
LAYER ?=
OVERLAYS ?=
TF_ARGS ?=

.PHONY: plan apply destroy plan-destroy refresh-kubeconfig refresh-postgres-env refresh-ansible-secrets bootstrap-svartalfheim-storage postgres-refresh postgres ragas-chunking-eval vllm-gateway-smoke ansible-minio-host ansible-minio-state ansible-minecraft-vm ansible-kubernetes-worker

RAGAS_CASES ?= evals/ragas/chunking_cases.jsonl
RAGAS_ARGS ?=

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
	pg_host="127.0.0.1"; \
	pg_port="15432"; \
	pg_db="app"; \
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
	echo "Updated $$env_file with Postgres connection details for local port-forward access at $$pg_host:$$pg_port/$$pg_db"; \
	echo "Use 'make postgres' to open a psql session through a temporary kubectl port-forward."; \
	echo "Run: . /home/vscode/.env"

postgres-refresh: refresh-postgres-env

refresh-ansible-secrets:
	@./scripts/ansible-fetch-secrets.sh

bootstrap-svartalfheim-storage:
	@./scripts/bootstrap-svartalfheim-storage.sh

postgres:
	@set -e; \
	if ! command -v psql >/dev/null 2>&1; then \
		echo "Error: psql is not installed in the devcontainer. Rebuild the devcontainer to pick up postgresql-client." >&2; \
		exit 1; \
	fi; \
	if ! command -v kubectl >/dev/null 2>&1; then \
		echo "Error: kubectl is required for the Postgres port-forward workflow." >&2; \
		exit 1; \
	fi; \
	$(MAKE) refresh-postgres-env >/dev/null; \
	. /home/vscode/.env; \
	port_forward_log="/tmp/postgres-port-forward.log"; \
	rm -f "$$port_forward_log"; \
	kubectl -n data port-forward svc/app-db-rw "$$DB_PORT:5432" >"$$port_forward_log" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	for _ in $$(seq 1 20); do \
		if grep -q "Forwarding from" "$$port_forward_log" 2>/dev/null; then \
			break; \
		fi; \
		if ! kill -0 "$$pf_pid" 2>/dev/null; then \
			cat "$$port_forward_log" >&2 || true; \
			echo "Error: failed to start kubectl port-forward for svc/app-db-rw in namespace data." >&2; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	if ! grep -q "Forwarding from" "$$port_forward_log" 2>/dev/null; then \
		cat "$$port_forward_log" >&2 || true; \
		echo "Error: timed out waiting for the Postgres port-forward to become ready." >&2; \
		exit 1; \
	fi; \
	PGPASSWORD="$$DB_PASS" psql -h "$$DB_HOST" -p "$$DB_PORT" -U "$$DB_USER" -d "$$DB_NAME"

ragas-chunking-eval:
	@set -e; \
	if ! command -v kubectl >/dev/null 2>&1; then \
		echo "Error: kubectl is required for the Postgres port-forward workflow." >&2; \
		exit 1; \
	fi; \
	secret_name="app-db-bootstrap"; \
	secret_namespace="data"; \
	DB_HOST="127.0.0.1"; \
	DB_PORT="15432"; \
	DB_NAME="app"; \
	DB_USER="$$(kubectl -n "$$secret_namespace" get secret "$$secret_name" -o jsonpath='{.data.username}' | base64 -d)"; \
	DB_PASS="$$(kubectl -n "$$secret_namespace" get secret "$$secret_name" -o jsonpath='{.data.password}' | base64 -d)"; \
	export DB_HOST DB_PORT DB_NAME DB_USER DB_PASS; \
	if [ -z "$$DB_USER" ] || [ -z "$$DB_PASS" ]; then \
		echo "Error: failed to load postgres credentials from $$secret_namespace/$$secret_name" >&2; \
		exit 1; \
	fi; \
	port_forward_log="/tmp/ragas-postgres-port-forward.log"; \
	rm -f "$$port_forward_log"; \
	kubectl -n data port-forward svc/app-db-rw "$$DB_PORT:5432" >"$$port_forward_log" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	for _ in $$(seq 1 20); do \
		if grep -q "Forwarding from" "$$port_forward_log" 2>/dev/null; then \
			break; \
		fi; \
		if ! kill -0 "$$pf_pid" 2>/dev/null; then \
			cat "$$port_forward_log" >&2 || true; \
			echo "Error: failed to start kubectl port-forward for svc/app-db-rw in namespace data." >&2; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	if ! grep -q "Forwarding from" "$$port_forward_log" 2>/dev/null; then \
		cat "$$port_forward_log" >&2 || true; \
		echo "Error: timed out waiting for the Postgres port-forward to become ready." >&2; \
		exit 1; \
	fi; \
	python3 evals/ragas/chunking_eval.py --cases "$(RAGAS_CASES)" $(RAGAS_ARGS)

vllm-gateway-smoke:
	@bash ./scripts/vllm-gateway-smoke.sh

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
