PROJECT_NAME = drbdtop
MAIN = main.go
BUILD_DIR =_build

DIRECTORIES = $(BUILD_DIR)

GO = go
BUILD_CMD = build -o $(BUILD_DIR)/$(PROJECT_NAME) $(PROJECT_NAME)/$(MAIN)

MKDIR = mkdir
MKDIR_FLAGS = -pv

RM = rm
RM_FLAGS = -rvf

.PHONY: make_directories

all: make_directories test build

make_directories:
	$(MKDIR) $(MKDIR_FLAGS) $(DIRECTORIES)  

test:
	$(GO) test "./pkg/..."

build: make_directories
	$(GO) $(BUILD_CMD)

clean:
	$(RM) $(RM_FLAGS) $(DIRECTORIES)
