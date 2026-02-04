.PHONY: test build package clean

PLUGIN_NAME := artist-nfo-metadata
WASM_FILE := plugin.wasm

test:
	go test -race ./...

build:
	tinygo build -opt=2 -scheduler=none -no-debug -o $(WASM_FILE) -target wasi -buildmode=c-shared .

package: build
	zip $(PLUGIN_NAME).ndp $(WASM_FILE) manifest.json

clean:
	rm -f $(WASM_FILE) $(PLUGIN_NAME).ndp

release: test
	@if [[ ! "${V}" =~ ^[0-9]+\.[0-9]+\.[0-9]+.*$$ ]]; then echo "Usage: make release V=X.X.X"; exit 1; fi
	go mod tidy
	@if [ -n "`git status -s`" ]; then echo "\n\nThere are pending changes. Please commit or stash first"; exit 1; fi
	git tag v${V}
	git push origin v${V} --no-verify
.PHONY: release