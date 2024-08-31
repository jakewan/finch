.DEFAULT_GOAL := local-dev-all

.PHONY: css-build
css-build:
	rm -f staticfiles/style.css
	stylus stylus --out staticfiles --prefix finch- --compress

.PHONY: go-doc
go-doc:
	godoc -http :8080

.PHONY: go-fmt
go-fmt:
	$(info Go formatting...)
	gofmt -d -s -w .

.PHONY: go-lint
go-lint:
	$(info Go linting...)
	golangci-lint run

.PHONY: go-test
go-test:
	$(info Running tests...)
	go test ./...

.PHONY: local-dev-all
local-dev-all: go-fmt go-test go-lint
