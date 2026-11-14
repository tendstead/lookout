TOOLS = docker compose -p lookout-tools -f compose.tools.yaml
.PHONY: deps fmt fmt-check test vet service-test web-check e2e build check up down

deps:
	$(TOOLS) run --rm go go mod tidy
	$(TOOLS) run --rm web npm ci --no-audit --no-fund
fmt:
	$(TOOLS) run --rm go gofmt -w cmd internal
	$(TOOLS) run --rm web npm run format
fmt-check:
	$(TOOLS) run --rm go sh -c 'test -z "$$(gofmt -l cmd internal)"'
	$(TOOLS) run --rm web npm run format:check
test:
	$(TOOLS) run --rm go go test -race -count=1 ./...
	$(TOOLS) run --rm web npm test
vet:
	$(TOOLS) run --rm go go vet ./...
service-test:
	sh scripts/test-service.sh
web-check:
	$(TOOLS) run --rm web npm run check
web-build:
	$(TOOLS) run --rm web npm run build
build:
	docker build -t lookout:local .
up:
	docker compose up -d --build --wait
down:
	docker compose down
check: fmt-check vet test service-test web-check e2e vuln workflow-lint
e2e: build
	sh scripts/test-browser.sh

vuln:
	$(TOOLS) run --rm go go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
	$(TOOLS) run --rm web npm audit --audit-level=high
workflow-lint:
	docker run --rm --cpus 1 --memory 128m --memory-swap 128m -v "$(CURDIR):/repo:ro" -w /repo rhysd/actionlint:1.7.12 -color

screenshots:
	docker compose -p lookout-browser -f compose.browser.yaml run --rm browser
