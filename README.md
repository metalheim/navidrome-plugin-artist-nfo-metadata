# Artist metadata from Kodi-style artist.nfo (navidrome-plugin)
A Navidrome plugin that can provide artist metadata to navidrome from Kodi-Style sidecar files ("artist.nfo").
The following metadata can be read from .nfo files and provided to navidrome
- ArtistBiography
- ArtistImages
- ArtistURL
- ArtistMBID
by implementing the navidrome `agent` functionalies that are exposed to the plugin.

## Configure Navidrome

```toml
[Plugins]
Enabled = true
Folder = "/path/to/plugins"

Agents = "artist-nfo-metadata,lastfm,deezer,spotify"
```

The plugin requires the `library` permission to read .nfo files from disk.
Please select which libraries the plugin is allowed to access in the Navidrome UI.
The Plugin will attempt to read an .nfo file in each library that is made available to it.

Navidrome mounts each library at `/libraries/<id>` inside the plugin.


## Build

```bash
tinygo build -o plugin.wasm -target wasip1 -buildmode=c-shared .
zip -j local-biography.ndp manifest.json plugin.wasm
```

## File Format

Example `artist.nfo`:

```xml
<artist>
  <name>FloFilz</name>
  <musicbrainzartistid>8f38496b-1b22-4633-a61a-cfdfdd1c9892</musicbrainzartistid>
  <thumb>https://f4.bcbits.com/img/0040023451_10.jpg</thumb>
  <biography>FloFilz (Florian Maier, *1991) ist ein deutscher Hip-Hop-Produzent und Violinist, der vor allem für seine atmosphärischen, jazz-basierten Beats im Lo-Fi- und Boom-Bap-Stil bekannt ist. Der in Belgien und Aachen aufgewachsene Künstler verbindet seine klassische Musikausbildung an der Geige mit modernem Sampling und gilt als eine der prägenden Figuren der europäischen Beat-Szene.</biography>
  <outline>FloFilz ist ein deutscher Hip-Hop-Produzent und Violinist aus Berlin</outline>
</artist>
```

## Notes

- The plugin first checks `<mount>/<artistname>/artist.nfo` exactly; if not present, it scans the library root for a directory matching the artist name case-insensitively.
- If you have multiple libraries, it searches all mounted libraries and returns the first match.
- Future improvements: configurable base path(s), alternative name normalization (spaces vs underscores), and optional recursive search.