TF ?= ./bin/tf
COMPONENT ?=
LAYER ?=
OVERLAYS ?=
TF_ARGS ?=

.PHONY: plan apply destroy plan-destroy refresh-join-token

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
