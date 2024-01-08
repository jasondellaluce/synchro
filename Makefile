VERSION := "0.0.0+$(shell git rev-parse HEAD)"

all: output

build_dir:
	mkdir -p build

output: build_dir
	go build -o build/synchro -ldflags="-X 'github.com/jasondellaluce/synchro/pkg/utils.ProjectVersion=$(VERSION)'"

clean:
	rm -fr build