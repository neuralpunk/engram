CGO_CFLAGS := -DSQLITE_ENABLE_FTS5
CGO_LDFLAGS := -lm
LDFLAGS := -s -w
export CGO_CFLAGS CGO_LDFLAGS

BINARY := engram
PREFIX := /usr/local/bin

.PHONY: build install test bench clean

build:
	go build -ldflags="$(LDFLAGS)" -trimpath -o $(BINARY) .

install: $(BINARY)
	install -m 755 $(BINARY) $(PREFIX)/$(BINARY)

test:
	go test ./...

bench:
	go test -bench=. -benchmem ./internal/db/

clean:
	rm -f $(BINARY)
