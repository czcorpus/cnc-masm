all: test build

build:
	manabuild masm3

install:
	cp ./masm3 /usr/local/bin

clean:
	rm masm3

test:
	manabuild -test masm3

.PHONY: clean install test