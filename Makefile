SOURCE_FILES?=./...
TEST_PATTERN?=.
TEST_OPTIONS?=

# Run all the tests
test:
	LC_ALL=C go test $(TEST_OPTIONS) -failfast -race -coverpkg=./... -covermode=atomic -coverprofile=coverage.txt $(SOURCE_FILES) -run $(TEST_PATTERN) -timeout=5m

# Run all the linters
lint:
	golangci-lint run ./...

# Build local binary
build:
	go build -ldflags="-X 'github.com/Mario-F/cloudflare-dyndns/cmd.Version=development'" -o bin/cloudflare-dyndns .

# Run goreleaser dry
godry:
	goreleaser release --snapshot --skip=publish --clean

.PHONY: test lint build godry
