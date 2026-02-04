go mod tidy
tinygo build -o plugin.wasm -target wasip1 -buildmode=c-shared .
zip -j artist-nfo-metadata.ndp manifest.json plugin.wasm
rm -f plugin.wasm
cp -f artist-nfo-metadata.ndp ../navidrome/plugins