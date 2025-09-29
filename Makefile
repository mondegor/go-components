-include Makefile.mk

deps:
	mrcmd go-dev deps

deps-upgrade:
	mrcmd go-dev get -u ./...
	mrcmd go-dev tidy

generate:
	mrcmd go-dev generate

lint:
	mrcmd go-dev fmt
	mrcmd go-dev fmti
	mrcmd go-dev fmti2
	mrcmd golangci-lint check

test:
	mrcmd go-dev test

test-report:
	mrcmd go-dev test-report

plantuml:
	mrcmd plantuml build-all

.PHONY: deps deps-upgrade generate lint test test-report plantuml