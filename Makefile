.PHONY: test benchmarks run build down logs wait-for-server ui-install ui-package ui-build ui-run api-test api-init

test:
	go test -v ./...

benchmarks:
	go test -bench=./... -run=^$ -benchmem

build:
	docker compose build

down:
	docker compose down

run: down build
	docker compose up -d

logs: run
	docker compose logs -f backend

wait-for-server:
	@echo "Waiting for server to be ready..."
	@until wget --spider -q http://localhost:8081/; do \
		echo "Server not yet available, waiting..."; \
		sleep 2; \
	done
	@echo "Server is up!"

ui-install:
	yarn workspaces focus @cate/ui

ui-package: ui-install
	yarn workspace @cate/ui build

ui-build: ui-package
    yarn prettier:check
	yarn build

ui-run: ui-build wait-for-server
	yarn workspace frontend run dev --host

api-init:
	python3 -m venv apitests/.venv
	. apitests/.venv/bin/activate && pip install -r apitests/requirements.txt

api-test: run wait-for-server
	. apitests/.venv/bin/activate && pytest apitests/
