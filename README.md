# Artist metadata from Kodi-style artist.nfo (navidrome-plugin)
A Navidrome plugin that can provide artist metadata to navidrome from Kodi-Style sidecar files ("artist.nfo").
The following metadata can be read from .nfo files and provided to navidrome by implementing the navidrome `agent` functionalies that are exposed to the plugin.
- ArtistBiography
- ArtistImages
- ArtistURL
- ArtistMBID


> [!NOTE]  
> The plugin checks `<mountPoint>[/subpath]/<artistName>/artist.nfo` exactly. 
> If your artistfolder doesn't match the artist name exactly, it will fail to deliver results.
> If you have multiple libraries, it searches all mounted libraries and returns the first match.

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

## File Format

> [!TIP]
>Plugin expects Kodi-Style xml files with `.nfo` extension. See here for details: https://kodi.wiki/view/NFO_files/Artists

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

## Build

```bash
tinygo build -o plugin.wasm -target wasip1 -buildmode=c-shared .
zip -j artist-nfo-metadata.ndp manifest.json plugin.wasm
```