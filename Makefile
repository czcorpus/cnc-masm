all: test build

build:
	manabuild masm3

install:
	cp ./masm3 /usr/local/bin

clean:
	rm masm3

test:
	go test ./...

rtest:
	go test -race ./...

.PHONY: clean install test