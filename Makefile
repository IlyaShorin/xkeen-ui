APP := xkeen-ui
CMD := ./cmd/xkeen-ui
BIN_DIR := ./bin
DIST_DIR := ./dist
LDFLAGS := -s -w
MIPS_GOTOOLCHAIN ?= go1.22.11

.PHONY: build test fmt dist dist-linux-mipsle dist-linux-mips dist-linux-arm64 clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP) $(CMD)

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal ./web

dist: dist-linux-mipsle dist-linux-mips dist-linux-arm64

dist-linux-mipsle:
	mkdir -p $(DIST_DIR)/linux-mipsle
	GOPROXY=https://proxy.golang.org,direct GOTOOLCHAIN=$(MIPS_GOTOOLCHAIN) CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/linux-mipsle/$(APP) $(CMD)

dist-linux-mips:
	mkdir -p $(DIST_DIR)/linux-mips
	GOPROXY=https://proxy.golang.org,direct GOTOOLCHAIN=$(MIPS_GOTOOLCHAIN) CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/linux-mips/$(APP) $(CMD)

dist-linux-arm64:
	mkdir -p $(DIST_DIR)/linux-arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/linux-arm64/$(APP) $(CMD)

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
