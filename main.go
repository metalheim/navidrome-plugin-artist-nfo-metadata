package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/metadata"
	"golang.org/x/text/encoding/charmap"
)

var (
	_ metadata.ArtistBiographyProvider = (*plugin)(nil)
	_ metadata.ArtistURLProvider       = (*plugin)(nil)
	_ metadata.ArtistMBIDProvider      = (*plugin)(nil)
	_ metadata.ArtistImagesProvider    = (*plugin)(nil)
)

type plugin struct{}

func init() {
	metadata.Register(&plugin{})
}

type subpathConfigEntry struct {
	LibraryId int    `json:"libraryId"`
	Subpath   string `json:"subpath"`
}

// artistNFO represents fields we care about from the Kodi-style artist.nfo
type artistNFO struct {
	XMLName             xml.Name `xml:"artist"`
	Name                string   `xml:"name"`
	MusicBrainzArtistID string   `xml:"musicbrainzartistid"`
	Thumb               string   `xml:"thumb"`
	Biography           string   `xml:"biography"`
	Outline             string   `xml:"outline"`
}

func (p *plugin) GetArtistBiography(input metadata.ArtistRequest) (*metadata.ArtistBiographyResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("artist-nfo-metadata: trying to fetch biography for %q from artist.nfo file", input.Name))
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name)
	if err != nil {
		return nil, fmt.Errorf("  artist.nfo file not found for artist %q", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %q", nfoPath)
	}

	if strings.TrimSpace(nfo.Biography) == "" {
		return nil, fmt.Errorf("  biography field is empty in %q", nfoPath)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("  found artist.nfo at %s. biography: %s", nfoPath, strings.TrimSpace(nfo.Biography)))
	return &metadata.ArtistBiographyResponse{Biography: nfo.Biography}, nil
}

func (p *plugin) GetArtistURL(input metadata.ArtistRequest) (*metadata.ArtistURLResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("artist-nfo-metadata: trying to fetch MusicBrainz URL for %q from artist.nfo file", input.Name))
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name)
	if err != nil {
		return nil, fmt.Errorf("  artist.nfo file not found for artist %q", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %q", nfoPath)
	}

	mbid := strings.TrimSpace(nfo.MusicBrainzArtistID)
	if mbid == "" {
		return nil, fmt.Errorf("  musicbrainz artist id not found in %q", nfoPath)
	}
	if _, err := uuid.Parse(mbid); err != nil {
		return nil, fmt.Errorf("  MBID found but invalid %q", mbid)
	}

	urlStr := "https://musicbrainz.org/artist/" + mbid
	pdk.Log(pdk.LogDebug, fmt.Sprintf("  found MBID %s in %s; returning URL %s", mbid, nfoPath, urlStr))
	return &metadata.ArtistURLResponse{URL: urlStr}, nil
}

func (p *plugin) GetArtistMBID(input metadata.ArtistMBIDRequest) (*metadata.ArtistMBIDResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("artist-nfo-metadata: trying to fetch MBID for %q from artist.nfo file", input.Name))
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name)
	if err != nil {
		return nil, fmt.Errorf("  artist.nfo file not found for artist %q", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %q", nfoPath)
	}

	mbid := strings.TrimSpace(nfo.MusicBrainzArtistID)
	if mbid == "" {
		return nil, fmt.Errorf("  musicbrainz artist id not found in %q", nfoPath)
	}
	if _, err := uuid.Parse(mbid); err != nil {
		return nil, fmt.Errorf("  MBID found but invalid %q", mbid)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("  returning MBID %s for %s", mbid, input.Name))
	return &metadata.ArtistMBIDResponse{MBID: mbid}, nil
}

func (p *plugin) GetArtistImages(input metadata.ArtistRequest) (*metadata.ArtistImagesResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("artist-nfo-metadata: trying to fetch artist images for %q from Kodi-style .nfo files", input.Name))
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name)
	if err != nil {
		return nil, fmt.Errorf("  artist.nfo file not found for artist %q", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %q", nfoPath)
	}

	thumb := strings.TrimSpace(nfo.Thumb)
	if thumb == "" {
		return nil, fmt.Errorf("  no thumb tag in %q", nfoPath)
	}

	// Validate URL (require http(s) and host)
	u, err := url.Parse(thumb)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("  invalid image url for artist %q", input.Name)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("  returning image %s from %s", thumb, nfoPath))
	return &metadata.ArtistImagesResponse{
		Images: []metadata.ImageInfo{
			{URL: thumb, Size: 0},
		},
	}, nil
}

// findNFO searches configured libraries and returns the first existing artist.nfo path.
// It applies the per-library subpath config if present and only checks the exact path:
// <mountPoint>[/subpath]/<artistName>/artist.nfo
func findNFO(artistName string) (string, error) {
	libraries, err := host.LibraryGetAllLibraries()
	if err != nil {
		return "", fmt.Errorf("  failed to get libraries: %w", err)
	}
	if len(libraries) == 0 {
		return "", errors.New("  no libraries available")
	}

	subpathMap := loadSubpathConfig()

	for _, lib := range libraries {
		if lib.MountPoint == "" {
			continue
		}

		subpath := ""
		if v, ok := subpathMap[int(lib.ID)]; ok {
			subpath = strings.Trim(v, "/")
		}

		nfoPath := filepath.Join(lib.MountPoint, subpath, artistName, "artist.nfo")

		// Check exact match first
		if fi, err := os.Stat(nfoPath); err == nil && !fi.IsDir() {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  nfo found at (exact): %s", nfoPath))
			return nfoPath, nil
		}

		// Probe parent dir for subfolders (case-insensitive search)
		parent := filepath.Join(lib.MountPoint, subpath)
		artistDir, ok := findMatchingDir(parent, artistName)
		if !ok {
			// not found in this library, continue to next lib
			continue
		}

		nfoPath = filepath.Join(artistDir, "artist.nfo")
		if fi, err := os.Stat(nfoPath); err == nil && !fi.IsDir() {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  nfo found at (case-insensitive): %s", nfoPath))
			return nfoPath, nil
		}

		// If artistDir matched but artist.nfo doesn't exist, continue to next lib
		continue
	}

	return "", os.ErrNotExist
}

// findMatchingDir returns the actual directory path under parent whose name
// matches wantedName case-insensitively. If none found, returns ("", false).
func findMatchingDir(parent, wantedName string) (string, bool) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), wantedName) {
			return filepath.Join(parent, e.Name()), true
		}
	}
	return "", false
}

// readArtistNFO parses the artist.nfo and returns the artistNFO struct.
func readArtistNFO(nfoPath string) (artistNFO, bool) {
	var empty artistNFO
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  failed to read %s: %v", nfoPath, err))
		return empty, false
	}
	a, err := parseArtistFromNFO(data)
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  failed to parse %s: %v", nfoPath, err))
		return empty, false
	}
	return a, true
}

// parseArtistFromNFO attempts to unmarshal XML from the provided bytes. It handles
// common problematic encodings by trying a few decoders if raw UTF-8 parsing fails.
func parseArtistFromNFO(data []byte) (artistNFO, error) {
	var a artistNFO

	// Strip optional UTF-8 BOM
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	// Quick try: assume data is valid UTF-8
	if err := xml.Unmarshal(data, &a); err == nil {
		return a, nil
	}

	// Try common legacy encodings used in NFO files
	decoders := []struct {
		name string
		dec  *charmap.Charmap
	}{
		{"windows-1252", charmap.Windows1252},
		{"iso-8859-1", charmap.ISO8859_1},
	}

	for _, d := range decoders {
		decoded, derr := d.dec.NewDecoder().Bytes(data)
		if derr != nil {
			continue
		}
		if err := xml.Unmarshal(decoded, &a); err == nil {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  parsed artist.nfo using %s decoder", d.name))
			return a, nil
		}
	}

	// Final fallback: convert bytes to string (invalid UTF-8 will be replaced)
	// and try again. This may lose/replace invalid runes but often succeeds.
	if err := xml.Unmarshal([]byte(string(data)), &a); err == nil {
		pdk.Log(pdk.LogDebug, "  parsed artist.nfo using fallback string conversion")
		return a, nil
	}

	return a, fmt.Errorf("  xml unmarshal failed for artist.nfo (attempted UTF-8, windows-1252, iso-8859-1, and fallback)")
}

// loadSubpathConfig reads pdk config key "subpaths" and returns a map[libraryId]subpath.
func loadSubpathConfig() map[int]string {
	result := make(map[int]string)

	cfgStr, ok := pdk.GetConfig("subpaths")
	if !ok || strings.TrimSpace(cfgStr) == "" {
		return result
	}

	var entries []subpathConfigEntry
	if err := json.Unmarshal([]byte(cfgStr), &entries); err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("  failed to parse subpaths config: %v", err))
		return result
	}

	for _, e := range entries {
		if e.LibraryId == 0 {
			continue
		}
		result[e.LibraryId] = strings.Trim(e.Subpath, "/")
	}
	return result
}

func main() {}