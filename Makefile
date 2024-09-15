-include Makefile.loc

.PHONY: deps
deps:
	mrcmd go-dev deps

.PHONY: deps-upgrade
deps-upgrade:
	mrcmd go-dev get -u ./...
	mrcmd go-dev tidy

.PHONY: generate
generate:
	mrcmd go-dev generate

.PHONY: fmt
fmt:
	mrcmd go-dev fmt

.PHONY: fmti
fmti:
	mrcmd go-dev fmti

.PHONY: lint
lint:
	mrcmd golangci-lint check

.PHONY: test
test:
	mrcmd go-dev test

.PHONY: test-report
test-report:
	mrcmd go-dev test-report

.PHONY: plantuml
plantuml:
	mrcmd plantuml build-all