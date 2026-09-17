# Where `make smoke` puts its throwaway build. Point it at an existing binary to
# smoke-test that one instead: make smoke GNOPLS=$(which gnopls)
GNOPLS ?= $(CURDIR)/.tmp/gnopls

.PHONY: install
install:
	go install -v .

.PHONY: test
test:
	go test -timeout 10m ./pkg/...

# Builds and actually starts the binary. `go build` alone cannot catch a
# package-level init() panic — see .github/scripts/smoke.sh.
.PHONY: smoke
smoke:
	@mkdir -p $(dir $(GNOPLS))
	go build -o $(GNOPLS) .
	.github/scripts/smoke.sh $(GNOPLS)

# Scoped to the Gno-owned code: internal/ is a vendored gopls tree.
.PHONY: lint
lint:
	gofmt -l main.go pkg internal/gnolang
	go vet ./pkg/... ./internal/gnolang/...

.PHONY: clean
clean:
	rm -rf .tmp dist
