.PHONY: check lint typecheck test

check: lint typecheck test

lint: go-vet
	@echo "lint: ok"

typecheck: go-vet ts-check
	@echo "typecheck: ok"

test: go-test fe-test
	@echo "test: ok"

go-vet:
	cd $(CURDIR) && go vet ./...

ts-check:
	cd $(CURDIR)/frontend && npx tsc --noEmit

go-test:
	cd $(CURDIR) && go test ./... -timeout 120s

fe-test:
	cd $(CURDIR)/frontend && npx vitest run
