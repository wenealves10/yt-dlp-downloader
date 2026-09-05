DEV_COMPOSE := docker compose --env-file api/.env -f compose.dev.yaml

.DEFAULT_GOAL := help

.PHONY: help dev dev-up dev-down dev-logs dev-ps dev-migrate dev-restart dev-browser-logs

help: ## Mostra os comandos do ambiente de desenvolvimento.
	@awk 'BEGIN {FS = ":.*##"}; /^[a-zA-Z_-]+:.*##/ {printf "%-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: dev-up ## Sobe todo o ambiente local e aplica as migrations.

dev-up: ## Sobe PostgreSQL, Redis, API, worker, navegador remoto e app em desenvolvimento.
	$(DEV_COMPOSE) up -d --build
	$(DEV_COMPOSE) ps
	@printf '\nApp: http://localhost:%s\nAPI: http://localhost:%s\n' "$${DEV_APP_PORT:-5173}" "$${DEV_API_PORT:-8081}"

dev-down: ## Para o ambiente local, preservando os dados dos volumes de desenvolvimento.
	$(DEV_COMPOSE) down

dev-logs: ## Acompanha os logs de todos os serviços locais.
	$(DEV_COMPOSE) logs -f

dev-ps: ## Exibe o estado dos serviços locais.
	$(DEV_COMPOSE) ps

dev-migrate: ## Executa novamente todas as migrations pendentes no PostgreSQL local.
	$(DEV_COMPOSE) run --rm migrate

dev-restart: ## Reinicia API, worker e app após mudanças que exigem novo processo.
	$(DEV_COMPOSE) restart server worker app

dev-browser-logs: ## Acompanha os logs do serviço de navegador remoto.
	$(DEV_COMPOSE) logs -f browser
