PROJECT_NAME = drbdtop
MAIN = drbdtop.go
VERSION=`git describe --tags --always --dirty`
OS=linux

GO = go
LDFLAGS = -ldflags "-X main.Version=${VERSION}"
BUILD_CMD = build $(LDFLAGS)
TEST_CMD = test "./pkg/..."


all: build

test:
	$(GO) $(TEST_CMD)

build:
	$(GO) $(BUILD_CMD)

install:
	$(GO) install

release:
	for os in ${OS}; do \
		GOOS=$$os GOARCH=amd64 go build ${LDFLAGS} -o ${PROJECT_NAME}-$$os-amd64; \
	done
