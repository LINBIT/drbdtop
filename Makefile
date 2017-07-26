PROJECT_NAME = drbdtop
MAIN = drbdtop.go
VERSION=`git describe --tags --always --dirty`

GO = go
LDFLAGS = -ldflags "-X main.Version=${VERSION}"
BUILD_CMD = build $(LDFLAGS)
TEST_CMD = test "./pkg/..."


all: test build

test:
	$(GO) $(TEST_CMD)

build:
	$(GO) $(BUILD_CMD)

install:
	$(GO) install
