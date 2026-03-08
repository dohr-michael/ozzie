.PHONY: build run-gateway run-ask test lint staticcheck clean build-plugins build-all check

build:
	go build -o build/ozzie ./cmd/ozzie

build-plugins:
	@mkdir -p build/plugins/calculator build/plugins/todo build/plugins/web-crawler build/plugins/patch
	cd plugins/calculator && tinygo build -target wasip1 -o ../../build/plugins/calculator/calculator.wasm .
	cp plugins/calculator/manifest.jsonc build/plugins/calculator/
	cd plugins/todo && tinygo build -target wasip1 -o ../../build/plugins/todo/todo.wasm .
	cp plugins/todo/manifest.jsonc build/plugins/todo/
	cd plugins/web-crawler && tinygo build -target wasip1 -o ../../build/plugins/web-crawler/web-crawler.wasm .
	cp plugins/web-crawler/manifest.jsonc build/plugins/web-crawler/
	cd plugins/patch && tinygo build -target wasip1 -o ../../build/plugins/patch/patch.wasm .
	cp plugins/patch/manifest.jsonc build/plugins/patch/

build-all: build build-plugins

run-gateway: build
	./build/ozzie gateway

run-ask: build
	./build/ozzie ask "Hello, who are you?"

test:
	go test ./...

lint:
	golangci-lint run ./...

staticcheck:
	~/go/bin/staticcheck ./...

clean:
	rm -rf build/

# Quality gates — all three must pass
check: build staticcheck test
