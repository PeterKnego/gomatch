# Load cloud credentials (AWS_* / HCLOUD_TOKEN / GOOGLE_*) from a gitignored
# .env if present, and export them into every recipe's environment so
# terraform/ansible pick them up automatically. Leading '-' makes .env optional.
# .env lines are KEY=value (bare value recommended).
-include .env
# Safety net: make's -include keeps surrounding quotes LITERALLY (unlike shell
# dotenv loaders), so a value like AWS_ACCESS_KEY_ID="abc" would be passed to
# terraform as `"abc"` (with quotes) and fail auth. Strip surrounding double
# quotes from known cred vars so both `KEY=abc` and `KEY="abc"` work.
_ENV_CREDS := HCLOUD_TOKEN AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN GOOGLE_PROJECT
$(foreach v,$(_ENV_CREDS),$(eval $(v) := $$(patsubst "%",%,$$($(v)))))
export

TF      := terraform -chdir=terraform
TFVARS  ?= terraform.tfvars
# Engine under test: go (gomatch) or java (javamatch, sibling checkout).
ENGINE  ?= go
ANSIBLE := ansible-playbook
SSH_KEY := $(shell awk -F'"' '/ssh_private_key_file/{print $$2}' $(TFVARS))

.PHONY: init up inventory bench bench-both status destroy ssh-node0

init:
	$(TF) init

up: ## provision the 3-node fleet + write the ansible inventory
	$(TF) apply -auto-approve -var-file=../$(TFVARS)
	$(MAKE) inventory

inventory:
	SSH_PRIVATE_KEY_FILE=$(SSH_KEY) ./inventory/terraform_to_inventory.sh

bench: ## configure hosts, start cluster + $(ENGINE) engine, run the sweep
	cd ansible && SSH_PRIVATE_KEY_FILE=$(SSH_KEY) $(ANSIBLE) gomatch.yml -e bench_engine=$(ENGINE)

bench-both: ## Go sweep, then Java sweep on the same fleet (fresh cluster state each)
	$(MAKE) bench ENGINE=go
	$(MAKE) bench ENGINE=java

status: ## list instances (cost guard — nothing auto-reaps, destroy yourself)
	@$(TF) output -json nodes | jq -r '.[] | "\(.name)\t\(.public_ip)\t\(.private_ip)\t\(.role)"'
	@echo "TTL guard: check instance uptime against ttl_hours in $(TFVARS)."

destroy: ## tear everything down
	$(TF) destroy -auto-approve -var-file=../$(TFVARS)
	rm -f inventory/hosts.yml

ssh-node0:
	@ip=$$($(TF) output -json nodes | jq -r '.[]|select(.role=="node0").public_ip'); \
	 user=$$($(TF) output -raw ssh_user); ssh -i $(SSH_KEY) $$user@$$ip
