.PHONY: build clean test

build: clean
	mkdir -p build
	go build -o build/dumbo .

clean:
	rm -rf build

test: 
	go test ./...