TF ?= ./bin/tf
COMPONENT ?=
LAYER ?=
OVERLAYS ?=
TF_ARGS ?=

.PHONY: plan apply destroy plan-destroy refresh-join-token refresh-postgres-env

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

refresh-join-token:
	@set -e; \
	env_file=".devcontainer/.env"; \
	join_cmd="$$(ssh yggdrasil 'kubeadm token create --print-join-command')"; \
	token="$$(printf '%s\n' "$$join_cmd" | sed -n 's/.*--token \([^ ]*\).*/\1/p')"; \
	if [ -z "$$token" ]; then \
		echo "Error: failed to parse kubeadm token from: $$join_cmd" >&2; \
		exit 1; \
	fi; \
	if grep -q '^TF_VAR_kubeadm_join_token=' "$$env_file"; then \
		sed -i "s|^TF_VAR_kubeadm_join_token=.*|TF_VAR_kubeadm_join_token='$$token'|" "$$env_file"; \
	else \
		printf "TF_VAR_kubeadm_join_token='%s'\n" "$$token" >> "$$env_file"; \
	fi; \
	echo "Updated $$env_file with TF_VAR_kubeadm_join_token=$$token"; \
	echo "Run: source /home/vscode/.env"

refresh-postgres-env:
	@set -e; \
	env_file=".devcontainer/.env"; \
	secret_name="app-db-bootstrap"; \
	secret_namespace="data"; \
	pg_host="app-db-rw.data.svc.cluster.local"; \
	pg_port="5432"; \
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
	echo "Updated $$env_file with Postgres connection details for $$pg_host:$$pg_port/$$pg_db"; \
	echo "Run: source /home/vscode/.env"

postgres:
	kubectl -n data port-forward svc/app-db-rw 5432:5432
