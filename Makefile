all: output

build_dir:
	mkdir -p build

output: build_dir
	go build -o build/synchross

clean:
	rm -fr build