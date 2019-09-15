NAME := dnstap-bgp
MAINTAINER:= Igor Novgorodov <igor@novg.net>
DESCRIPTION := DNSTap to BGP exporter
LICENSE := MPLv2

GO ?= go
DEP ?= dep
VERSION := $(shell cat VERSION)
OUT := .out
PACKAGE := github.com/blind-oracle/$(NAME)
ARCH := amd64

all: build

build:
	$(DEP) ensure
	rm -rf $(OUT)
	mkdir -p $(OUT)
	go build -ldflags "-s -w -X main.version=$(VERSION)"
	mv $(NAME) $(OUT)
	mkdir -p $(OUT)/root/etc/$(NAME)
	cp deploy/dnstap-bgp.conf $(OUT)/root/etc/$(NAME)/$(NAME).conf

deb:
	make build
	make build-deb

rpm:
	make build
	make build-rpm

build-deb:
	fpm -s dir -t deb -n $(NAME) -v $(VERSION) \
		--deb-priority optional \
		--category admin \
		--force \
		--url https://$(PACKAGE) \
		--description "$(DESCRIPTION)" \
		-m "$(MAINTAINER)" \
		--license "$(LICENSE)" \
		-a $(ARCH) \
		$(OUT)/$(NAME)=/usr/bin/$(NAME) \
		deploy/$(NAME).service=/usr/lib/systemd/system/$(NAME).service \
		deploy/$(NAME)=/etc/default/$(NAME) \
		$(OUT)/root/=/

build-rpm:
	fpm -s dir -t rpm -n $(NAME) -v $(VERSION) \
		--force \
		--rpm-compression bzip2 \
		--rpm-os linux \
		--url https://$(PACKAGE) \
		--description "$(DESCRIPTION)" \
		-m "$(MAINTAINER)" \
		--license "$(LICENSE)" \
		-a $(ARCH) \
		--config-files /etc/$(NAME)/$(NAME).conf \
		$(OUT)/$(NAME)=/usr/bin/$(NAME) \
		deploy/$(NAME).service=/usr/lib/systemd/system/$(NAME).service \
		deploy/$(NAME)=/etc/default/$(NAME) \
		$(OUT)/root/=/
