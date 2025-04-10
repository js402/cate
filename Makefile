.PHONY: test benchmarks run build down logs ui-install ui-package ui-build ui-run api-test api-init

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

yarn-wipe:
	echo "Removing Yarn PnP files..."
	rm -f .pnp.cjs .pnp.loader.mjs
	echo "Removing Yarn state files and unplugged directory..."
	rm -rf .yarn/unplugged
	rm -f .yarn/install-state.gz
	echo "Removing node_modules directories..."
	rm -rf node_modules packages/*/node_modules frontend/node_modules
	echo "Running yarn install..."
	yarn install

ui-install:
	yarn workspaces focus @cate/ui frontend

ui-package: ui-install
	yarn workspace @cate/ui build

ui-build: ui-package
    yarn prettier:check
	yarn build

ui-run: ui-build
	yarn workspace frontend dev --host

api-init:
	python3 -m venv apitests/.venv
	. apitests/.venv/bin/activate && pip install -r apitests/requirements.txt

api-test: run
	. apitests/.venv/bin/activate && pytest apitests/
