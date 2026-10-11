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
	mrcmd go-dev lint

lint-verify:
	mrcmd go-dev lint-verify

test:
	mrcmd go-dev test

test-report:
	mrcmd go-dev test-report

plantuml:
	mrcmd plantuml build-all

.PHONY: deps deps-upgrade generate lint lint-verify test test-report plantuml
