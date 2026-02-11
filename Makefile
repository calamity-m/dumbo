.PHONY: build clean

build: clean
	mkdir -p build
	go build -o build/dumbo .

clean:
	rm -rf build