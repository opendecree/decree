.PHONY: up setup test down clean help

EXAMPLES := quickstart feature-flags live-config multi-tenant optimistic-concurrency schema-lifecycle environment-bootstrap config-validation

## up: Start the decree server (postgres + redis + migrations + service)
up:
	cd .. && docker compose up -d --wait service

## setup: Seed example schema, tenant, and config (writes .tenant-id)
setup: up
	cd setup && go run .

## test: Run all examples as tests (run 'make setup' first)
test:
	@for dir in $(EXAMPLES); do \
		echo "=== $$dir ==="; \
		(cd $$dir && go test -tags=example -v -count=1 ./...) || exit 1; \
	done

## down: Stop the server and remove volumes
down:
	cd .. && docker compose down -v

## clean: Remove generated files
clean:
	rm -f .tenant-id

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
