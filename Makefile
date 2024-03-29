# supress output, run `make XXX V=` to be verbose
V := @

OUT_DIR := ./build
VULCAN_OUT := $(OUT_DIR)/vulcan
VULCAN_MAIN_PKG := ./cmd/vulcan

REFERRAL_OUT := $(OUT_DIR)/referral
REFERRAL_MAIN_PKG := ./cmd/referral

VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

LDFLAGS = -s -w -X github.com/cosmos/cosmos-sdk/version.Name=decentr \
	-X github.com/Decentr-net/vulcan/internal/health.version=$(VERSION) \
	-X github.com/Decentr-net/vulcan/internal/health.commit=$(COMMIT)

GOBIN := $(shell go env GOPATH)/bin

LINTER_NAME := golangci-lint
LINTER_VERSION := v1.29.0

MOCKGEN_NAME := mockgen
MOCKGEN_VERSION := v1.4.3

SWAGGER_NAME := swagger
SWAGGER_VERSION := v0.26.0

default: build

.PHONY: build
build:
	@echo BUILDING $(VULCAN_OUT)
	$(V) CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o $(VULCAN_OUT) $(VULCAN_MAIN_PKG)
	@echo DONE

	@echo BUILDING $(REFERRAL_OUT)
	$(V) CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o $(REFERRAL_OUT) $(REFERRAL_MAIN_PKG)
	@echo DONE

.PHONY: linux
linux: export GOOS := linux
linux: export GOARCH := amd64
linux: LINUX_VULCAN_OUT := $(VULCAN_OUT)-$(GOOS)-$(GOARCH)
linux: LINUX_REFERRAL_OUT := $(REFERRAL_OUT)-$(GOOS)-$(GOARCH)
linux:
	@echo BUILDING $(LINUX_VULCAN_OUT)
	$(V) CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o $(LINUX_VULCAN_OUT) $(VULCAN_MAIN_PKG)
	@echo DONE

	@echo BUILDING $(LINUX_REFERRAL_OUT)
	$(V) CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o $(LINUX_REFERRAL_OUT) $(REFERRAL_MAIN_PKG)
	@echo DONE

.PHONY: image
image:
	docker build -t vulcan-local -f scripts/Dockerfile .

.PHONY: clean
clean:
	$(V)rm -rf $(OUT_DIR)

.PHONY: test
test: GO_TEST_FLAGS := -race
test:
	$(V)go test -ldflags "$(LDFLAGS)" -mod=vendor -v $(GO_TEST_FLAGS) $(GO_TEST_TAGS) ./...

.PHONY: fulltest
fulltest: GO_TEST_TAGS := --tags=integration
fulltest: test

.PHONY: lint
lint: check-linter-version
	$(V)$(LINTER_NAME) run --config configs/.golangci.yml

.PHONY: generate
generate: check-all
	$(V)go generate -mod=vendor -x ./...


.PHONY: vendor
vendor:
	$(V)go mod tidy
	$(V)go mod vendor

.PHONY: install-linter
install-linter: LINTER_INSTALL_PATH := $(GOBIN)/$(LINTER_NAME)
install-linter:
	@echo INSTALLING $(LINTER_INSTALL_PATH) $(LINTER_VERSION)
	$(V)curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(GOBIN) $(LINTER_VERSION)
	@echo DONE

.PHONY: install-mockgen
install-mockgen: MOCKGEN_INSTALL_PATH := $(GOBIN)/$(MOCKGEN_NAME)
install-mockgen:
	@echo INSTALLING $(MOCKGEN_INSTALL_PATH) $(MOCKGEN_NAME)
	# we need to change dir to use go modules without updating repo deps
	$(V)cd $(TMPDIR) && GO111MODULE=on go get github.com/golang/mock/mockgen@$(MOCKGEN_VERSION)
	@echo DONE

.PHONY: install-swagger
install-swagger: SWAGGER_INSTALL_PATH := $(GOBIN)/$(SWAGGER_NAME)
install-swagger: UNAME_OS := $(shell uname -s)
install-swagger:
	@echo INSTALLING $(SWAGGER_INSTALL_PATH) $(SWAGGER_VERSION)
	# we need to change dir to use go modules without updating repo deps
	$(V)cd $(TMPDIR) && GO111MODULE=on go get github.com/go-swagger/go-swagger/cmd/swagger@$(SWAGGER_VERSION)
	@echo DONE

.PHONY: check-linter-version
check-linter-version: ACTUAL_LINTER_VERSION := $(shell $(LINTER_NAME) --version 2>/dev/null | awk '{print $$4}')
check-linter-version:
	$(V) [ -z $(ACTUAL_LINTER_VERSION) ] && \
	 echo 'Linter is not installed, run `make install-linter`' && \
	 exit 1 || true

	$(V)if [ v$(ACTUAL_LINTER_VERSION) != $(LINTER_VERSION) ] ; then \
		echo $(LINTER_NAME) is version v$(ACTUAL_LINTER_VERSION), want $(LINTER_VERSION) ; \
		echo 'Make sure $$GOBIN has precedence in $$PATH and' \
		'run `make install-linter` to install the correct version' ; \
        exit 1 ; \
	fi

.PHONY: check-mockgen-version
check-mockgen-version: ACTUAL_MOCKGEN_VERSION := $(shell $(MOCKGEN_NAME) --version 2>/dev/null)
check-mockgen-version:
	$(V) [ -z $(ACTUAL_MOCKGEN_VERSION) ] && \
	 echo 'Mockgen is not installed, run `make install-mockgen`' && \
	 exit 1 || true

	$(V)if [ $(ACTUAL_MOCKGEN_VERSION) != $(MOCKGEN_VERSION) ] ; then \
		echo $(MOCKGEN_NAME) is version $(ACTUAL_MOCKGEN_VERSION), want $(MOCKGEN_VERSION) ; \
		echo 'Make sure $$GOBIN has precedence in $$PATH and' \
		'run `make install-mockgen` to install the correct version' ; \
        exit 1 ; \
	fi

.PHONY: check-swagger-version
check-swagger-version: ACTUAL_SWAGGER_VERSION := $(shell $(SWAGGER_NAME) version 2>/dev/null | grep version | cut -c 10-17)
# hack version, see https://github.com/go-swagger/go-swagger/issues/1712#issuecomment-422981313
check-swagger-version: WANT_SWAGGER_VERSION := $(SWAGGER_VERSION)
check-swagger-version:
	$(V) [ -z $(ACTUAL_SWAGGER_VERSION) ] && \
	 echo 'Swagger is not installed, run `make install-swagger`' && \
	 exit 1 || true

	$(V)if [ $(ACTUAL_SWAGGER_VERSION) != $(WANT_SWAGGER_VERSION) ] ; then \
		echo $(SWAGGER_NAME) is version $(ACTUAL_SWAGGER_VERSION), want $(WANT_SWAGGER_VERSION) ; \
		echo 'Make sure $$GOBIN has precedence in $$PATH and' \
		'run `make install-swagger` to install the correct version' ; \
        exit 1 ; \
	fi

.PHONY: check-all
check-all: check-swagger-version check-mockgen-version check-swagger-version

.PHONY: install-all
install-all: install-linter install-mockgen install-swagger