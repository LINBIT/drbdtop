PROJECT_NAME = drbdtop
MAIN = main.go
VERSION=`git describe --tags --always --dirty`
PREFIX=/usr/local
BIN_DIR = bin

BUILD_DIR =_build

DIRECTORIES = $(BUILD_DIR)

GO = go
LDFLAGS = -ldflags "-X main.Version=${VERSION}"
BUILD_CMD = build $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME) $(PROJECT_NAME)/$(MAIN)
TEST_CMD = test "./pkg/..."

MKDIR = mkdir
MKDIR_FLAGS = -pv

CP = cp
CP_FLAGS = --interactive

CHMOD = chmod
CHMOD_FLAGS = a+x

RM = rm
RM_FLAGS = -rvf

.PHONY: make_directories

all: make_directories test build

make_directories:
	$(MKDIR) $(MKDIR_FLAGS) $(DIRECTORIES)

test:
	$(GO) $(TEST_CMD)

build: make_directories
	$(GO) $(BUILD_CMD)

install:
	$(CP) $(CP_FLAGS) $(BUILD_DIR)/$(PROJECT_NAME) $(PREFIX)/$(BIN_DIR) && $(CHMOD) $(CHMOD_FLAGS) $(PREFIX)/$(BIN_DIR)/$(PROJECT_NAME)

clean:
	$(RM) $(RM_FLAGS) $(DIRECTORIES)
