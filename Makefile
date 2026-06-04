.PHONY: up up-prod desktop down build images package-desktop test sim seed assert-queue tidy

COMPOSE = docker compose -f deploy/docker-compose.yml
COMPOSE_PROD = docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env
SCENARIO ?= queue_fairness

up:
	$(COMPOSE) --profile loadtest up -d --build postgres redis api worker loadtest-runner

up-prod:
	@test -f deploy/.env || (echo "Copy deploy/.env.example to deploy/.env and set secrets first." && exit 1)
	$(COMPOSE_PROD) up -d --build

desktop:
	chmod +x scripts/run-traffic.sh
	./scripts/run-traffic.sh

images:
	chmod +x scripts/build-images.sh
	./scripts/build-images.sh

package-desktop:
	chmod +x scripts/package-desktop.sh
	./scripts/package-desktop.sh

down:
	$(COMPOSE) down

build:
	cd deploy && docker compose -f docker-compose.yml build api

test:
	go test ./...

tidy:
	go mod tidy

seed:
	chmod +x scripts/seed.sh
	./scripts/seed.sh

sim:
	$(COMPOSE) --profile loadtest up -d api worker loadtest-runner
	@sleep 5
	$(COMPOSE) --profile loadtest run --rm \
		-e K6_SCENARIO=$(SCENARIO) \
		-e K6_VUS_MAX=$${K6_VUS_MAX:-500} \
		-e K6_RAMP_UP=$${K6_RAMP_UP:-10s} \
		-e K6_STEADY=$${K6_STEADY:-30s} \
		-e K6_RAMP_DOWN=$${K6_RAMP_DOWN:-10s} \
		-e SALE_ID=$${SALE_ID:-} \
		k6 run /scripts/scenarios/$(SCENARIO).js

assert-queue:
	go run ./cmd/assert-queue -sale "$(SALE_ID)"

logs:
	$(COMPOSE) logs -f api worker
