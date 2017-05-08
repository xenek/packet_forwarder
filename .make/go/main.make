# Programs
GO = go
GOLINT = golint

# License keys
# To add a license key to the binary, specify the variable name using
# LICENSE_KEY_VAR, and add a LICENSE_KEY_FILE variable to the build.
LICENSE_KEY_VAR ?= "main.licenseKey"
LICENSE_KEY_STR ?= $(shell cat "$(LICENSE_KEY_FILE)" 2>/dev/null | head -n -1 | tail -n +2 | tr -d '\n')

# Flags
## go
GO_FLAGS = -a
ifeq ($(HAL_CHOICE),dummy)
	GO_ENV = CGO_ENABLED=0
else
	GO_ENV = CGO_ENABLED=1
endif

## golint
GOLINT_FLAGS = -set_exit_status

## test
GO_TEST_FLAGS = -cover

## coverage
GO_COVER_FILE = coverage.out
GO_COVER_DIR  = .coverage

# Filters

## select only go files
only_go = grep '.go$$'

## select/remove vendored files
no_vendor = grep -v 'vendor'
only_vendor = grep 'vendor'

## select/remove mock files
no_mock = grep -v '_mock.go'
only_mock = grep '_mock.go'

## select/remove protobuf generated files
no_pb = grep -Ev '.pb.go$$|.pb.gw.go$$'
only_pb = grep -E '.pb.go$$|.pb.gw.go$$'

## select/remove test files
no_test = grep -v '_test.go$$'
only_test = grep '_test.go$$'

## filter files to packages
to_packages = sed 's:/[^/]*$$::' | sort | uniq

## make packages local (prefix with ./)
to_local = sed 's:^:\./:'


# Selectors

## find all go files
GO_FILES = find . -name '*.go' | grep -v '.git'

## local go packages
GO_PACKAGES = $(GO_FILES) | $(no_vendor) | $(to_packages)

## external go packages (in vendor)
EXTERNAL_PACKAGES = $(GO_FILES) | $(only_vendor) | $(to_packages)

## staged local packages
STAGED_PACKAGES = $(STAGED_FILES) | $(only_go) | $(no_vendor) | $(to_packages) | $(to_local)

## packages for testing
TEST_PACKAGES = $(GO_FILES) | $(no_vendor) | $(only_test) | $(to_packages)

# Rules

## get tools required for development
go.dev-deps:
	@$(log) "fetching go tools"
	@command -v govendor > /dev/null || ($(log) Installing govendor && $(GO) get -v -u github.com/kardianos/govendor)
	@command -v golint > /dev/null || ($(log) Installing golint && $(GO) get -v -u github.com/golang/lint/golint)

## install dependencies
go.deps:
	@$(log) "fetching go dependencies"
	@govendor sync -v

## install packages for faster rebuilds
go.install:
	@$(log) "installing go packages"
	@$(EXTERNAL_PACKAGES) | xargs $(GO) install -v

## clean build files
go.clean:
	@$(log) "cleaning release dir" [rm -rf $(RELEASE_DIR)]
	@rm -rf $(RELEASE_DIR)

## clean dependencies
go.clean-deps:
	@$(log) "cleaning go dependencies" [rm -rf vendor/*/]
	@rm -rf vendor/*/

## run tests
go.test:
	@$(log) testing `$(TEST_PACKAGES) | $(count)` go packages
	@$(GO) test $(GO_TEST_FLAGS) `$(TEST_PACKAGES)`

## clean cover files
go.cover.clean:
	rm -rf $(GO_COVER_DIR) $(GO_COVER_FILE)

## package coverage
$(GO_COVER_DIR)/%.out: GO_TEST_FLAGS=-cover -coverprofile="$(GO_COVER_FILE)"
$(GO_COVER_DIR)/%.out: %
	@$(log) testing "$<"
	@mkdir -p `dirname "$(GO_COVER_DIR)/$<"`
	@$(GO) test -cover -coverprofile="$@" "./$<"

## project coverage
$(GO_COVER_FILE): go.cover.clean $(patsubst ./%,./$(GO_COVER_DIR)/%.out,$(shell $(TEST_PACKAGES)))
	@echo "mode: set" > $(GO_COVER_FILE)
	@cat $(patsubst ./%,./$(GO_COVER_DIR)/%.out,$(shell $(TEST_PACKAGES))) | grep -vE "mode: set" | sort >> $(GO_COVER_FILE)

# vim: ft=make
