-include Makefile.mk

deps:
	mrcmd go-dev deps

deps-upgrade:
	mrcmd go-dev get -u ./...
	mrcmd go-dev tidy

generate:
	mrcmd go-dev generate

fmt:
	mrcmd go-dev fmt

fmti:
	mrcmd go-dev fmti

lint:
	mrcmd golangci-lint check

test:
	mrcmd go-dev test

test-report:
	mrcmd go-dev test-report

plantuml:
	mrcmd plantuml build-all

.PHONY: deps deps-upgrade generate fmt fmti lint test test-report plantuml