.PHONY: build run test lint integration load docker-build docker-run check clean

BIN := bin/goboxd
COMPOSE ?= docker compose
GOBOXD_URL ?= http://localhost:8080

build:
	@mkdir -p bin
	go build -trimpath -o $(BIN) ./cmd/goboxd

run: build
	./$(BIN)

test:
	go test ./...

lint:
	go vet ./...
	@command -v staticcheck >/dev/null && staticcheck ./... || echo "staticcheck not installed; skipping"

# integration: bring the container up, wait for /readyz, run tests/ with
# the integration build tag against the live server, then tear down.
integration:
	$(COMPOSE) up -d --build
	@echo "waiting for $(GOBOXD_URL)/readyz ..."
	@for i in $$(seq 1 60); do \
		if curl -fsS $(GOBOXD_URL)/readyz >/dev/null 2>&1; then break; fi; \
		sleep 1; \
	done
	GOBOXD_URL=$(GOBOXD_URL) go test -tags=integration -v ./tests/...
	$(COMPOSE) down

load:
	@bash scripts/load.sh

# check: ultimate pre-submission verification (phases A through H).
check:
	@bash testdata/check.sh

docker-build:
	docker build -t goboxd:dev .

docker-run:
	$(COMPOSE) up --build

clean:
	rm -rf bin dist
