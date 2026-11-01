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
	"unicode"

	"github.com/google/uuid"
	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/metadata"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
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
		pdk.Log(pdk.LogDebug, "  empty artist name")
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name, input.MBID)
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file not found for artist %s: %v", input.Name, err))
		return nil, fmt.Errorf("  artist.nfo file not found for artist %s", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file couldn't be read at %s", nfoPath))
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %s", nfoPath)
	}

	if strings.TrimSpace(nfo.Biography) == "" {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  biography field is empty in %s", nfoPath))
		return nil, fmt.Errorf("  biography field is empty in %s", nfoPath)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("  found artist.nfo at %s. biography: %s", nfoPath, strings.TrimSpace(nfo.Biography)))
	return &metadata.ArtistBiographyResponse{Biography: nfo.Biography}, nil
}

func (p *plugin) GetArtistURL(input metadata.ArtistRequest) (*metadata.ArtistURLResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("artist-nfo-metadata: trying to fetch MusicBrainz URL for %s from artist.nfo file", input.Name))
	if strings.TrimSpace(input.Name) == "" {
		pdk.Log(pdk.LogDebug, "  empty artist name")
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name, input.MBID)
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file not found for artist %s: %v", input.Name, err))
		return nil, fmt.Errorf("  artist.nfo file not found for artist %s", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file couldn't be read at %s", nfoPath))
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %s", nfoPath)
	}

	mbid := strings.TrimSpace(nfo.MusicBrainzArtistID)
	if mbid == "" {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  musicbrainz artist id not found in %s", nfoPath))
		return nil, fmt.Errorf("  musicbrainz artist id not found in %s", nfoPath)
	}
	if _, err := uuid.Parse(mbid); err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  MBID found but invalid %s: %v", mbid, err))
		return nil, fmt.Errorf("  MBID found but invalid %s", mbid)
	}

	urlStr := "https://musicbrainz.org/artist/" + mbid
	pdk.Log(pdk.LogDebug, fmt.Sprintf("  found MBID %s in %s; returning URL %s", mbid, nfoPath, urlStr))
	return &metadata.ArtistURLResponse{URL: urlStr}, nil
}

func (p *plugin) GetArtistMBID(input metadata.ArtistMBIDRequest) (*metadata.ArtistMBIDResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("artist-nfo-metadata: trying to fetch MBID for %s from artist.nfo file", input.Name))
	if strings.TrimSpace(input.Name) == "" {
		pdk.Log(pdk.LogDebug, "  empty artist name")
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name, "")
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file not found for artist %s: %v", input.Name, err))
		return nil, fmt.Errorf("  artist.nfo file not found for artist %s", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file couldn't be read at %s", nfoPath))
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %s", nfoPath)
	}

	mbid := strings.TrimSpace(nfo.MusicBrainzArtistID)
	if mbid == "" {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  musicbrainz artist id not found in %s", nfoPath))
		return nil, fmt.Errorf("  musicbrainz artist id not found in %s", nfoPath)
	}
	if _, err := uuid.Parse(mbid); err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  MBID found but invalid %s: %v", mbid, err))
		return nil, fmt.Errorf("  MBID found but invalid %s", mbid)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("  returning MBID %s for %s", mbid, input.Name))
	return &metadata.ArtistMBIDResponse{MBID: mbid}, nil
}

func (p *plugin) GetArtistImages(input metadata.ArtistRequest) (*metadata.ArtistImagesResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("artist-nfo-metadata: trying to fetch artist images for %s from Kodi-style .nfo files", input.Name))
	if strings.TrimSpace(input.Name) == "" {
		pdk.Log(pdk.LogDebug, "  empty artist name")
		return nil, errors.New("  empty artist name")
	}

	nfoPath, err := findNFO(input.Name, input.MBID)
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file not found for artist %s: %v", input.Name, err))
		return nil, fmt.Errorf("  artist.nfo file not found for artist %s", input.Name)
	}

	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo file couldn't be read at %s", nfoPath))
		return nil, fmt.Errorf("  artist.nfo file couldn't be read at %s", nfoPath)
	}

	thumb := strings.TrimSpace(nfo.Thumb)
	if thumb == "" {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  no thumb tag in %s", nfoPath))
		return nil, fmt.Errorf("  no thumb tag in %s", nfoPath)
	}

	// Validate URL (require http(s) and host)
	u, err := url.Parse(thumb)
	if err != nil || u.Scheme == "" || u.Host == "" {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  invalid image url in %q for artist %s: %q (parseErr=%v, scheme=%q, host=%q)", nfoPath, input.Name, thumb, err, u.Scheme, u.Host))
		return nil, fmt.Errorf("  invalid image url for artist %s", input.Name)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("  returning image %s from %s", thumb, nfoPath))
	return &metadata.ArtistImagesResponse{
		Images: []metadata.ImageInfo{
			{URL: thumb, Size: 0},
		},
	}, nil
}

// findNFO searches configured libraries and returns the first existing artist.nfo path.
// It applies the per-library subpath config if present and checks, in this order:
// the exact path <mountPoint>[/subpath]/<artistName>/artist.nfo, a directory whose
// name matches case-insensitively, and a directory named with a filesystem-safe
// spelling of the name (see nameVariants). A match through a spelling is only
// accepted when the nfo does not contradict it: its <name> (if any) must be the
// artist's name and its <musicbrainzartistid> (if any) must equal mbid when both
// are known.
func findNFO(artistName, mbid string) (string, error) {
	libraries, err := host.LibraryGetAllLibraries()
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  failed to get libraries: %v", err))
		return "", fmt.Errorf("  failed to get libraries: %w", err)
	}
	if len(libraries) == 0 {
		pdk.Log(pdk.LogDebug, "  no libraries available")
		return "", errors.New("  no libraries available")
	}

	subpathMap := loadSubpathConfig()

	for _, lib := range libraries {
		if lib.MountPoint == "" {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  skipping library %d: empty mountPoint", lib.ID))
			continue
		}

		subpath := ""
		if v, ok := subpathMap[int(lib.ID)]; ok {
			subpath = strings.Trim(v, "/")
		}

		nfoPath := filepath.Join(lib.MountPoint, subpath, artistName, "artist.nfo")

		// Check exact match first
		if fi, err := os.Stat(nfoPath); err == nil && !fi.IsDir() {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo found at (exact): %s", nfoPath))
			return nfoPath, nil
		} else if err != nil {
			// This is the key diagnostic: why did stat fail?
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  stat failed for %s: %v", nfoPath, err))
		} else if fi != nil && fi.IsDir() {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  path exists but is a directory (expected file): %q", nfoPath))
		}

		// Probe parent dir for subfolders (case-insensitive search, then the
		// filesystem-safe spellings of the name, see nameVariants)
		parent := filepath.Join(lib.MountPoint, subpath)
		artistDir, ok := findMatchingDir(parent, artistName)
		if !ok {
			// not found in this library, continue to next lib
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  couldn't find artist dir %s in %s", artistName, parent))
			continue
		}

		nfoPath = filepath.Join(artistDir, "artist.nfo")
		pdk.Log(pdk.LogTrace, fmt.Sprintf("  Probing nfo at: %s", nfoPath))
		if fi, err := os.Stat(nfoPath); err == nil && !fi.IsDir() {
			if !sameName(filepath.Base(artistDir), artistName) && !nfoBelongsTo(nfoPath, artistName, mbid) {
				pdk.Log(pdk.LogInfo, fmt.Sprintf("  %s is named like a spelling of %q but its artist.nfo describes another artist, ignored", artistDir, artistName))
				continue
			}
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  artist.nfo found at (directory match): %s", nfoPath))
			return nfoPath, nil
		}

		// If artistDir matched but artist.nfo doesn't exist, continue to next lib
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  matched artist dir %q but artist.nfo not found", artistDir))
		continue
	}

	return "", os.ErrNotExist
}

// findMatchingDir returns the actual directory path under parent whose name
// matches wantedName case-insensitively (after NFC normalisation of both, so a
// folder created on macOS matches a name tagged on Linux). When no directory
// matches, the filesystem-safe spellings from nameVariants are tried in order, so
// an artist "AC/DC" is found in "ACDC" and "Mötley Crüe" in "Motley Crue". If
// none found, returns ("", false).
func findMatchingDir(parent, wantedName string) (string, bool) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  failed to read dir %q: %v", parent, err))
		return "", false
	}
	if dir, ok := matchDir(parent, entries, wantedName); ok {
		return dir, true
	}
	for _, variant := range nameVariants(wantedName) {
		if dir, ok := matchDir(parent, entries, variant); ok {
			pdk.Log(pdk.LogInfo, fmt.Sprintf("  artist dir for %q matched through filesystem-safe spelling %q", wantedName, variant))
			return dir, true
		}
	}
	return "", false
}

// matchDir returns the directory among entries whose name is wantedName
// (see sameName).
func matchDir(parent string, entries []os.DirEntry, wantedName string) (string, bool) {
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if sameName(e.Name(), wantedName) {
			return filepath.Join(parent, e.Name()), true
		}
	}
	return "", false
}

// sameName compares two names case-insensitively after NFC normalisation.
func sameName(a, b string) bool {
	return strings.EqualFold(norm.NFC.String(a), norm.NFC.String(b))
}

// nfoBelongsTo reports whether the artist.nfo at nfoPath can describe the
// artist: its <name>, when present, must be artistName, and its
// <musicbrainzartistid>, when present and when mbid is known, must be mbid. An
// unreadable file is not a match.
func nfoBelongsTo(nfoPath, artistName, mbid string) bool {
	nfo, ok := readArtistNFO(nfoPath)
	if !ok {
		return false
	}
	if name := strings.TrimSpace(nfo.Name); name != "" && !sameName(name, artistName) {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  %s names %q, not %q", nfoPath, name, artistName))
		return false
	}
	nfoMBID := strings.TrimSpace(nfo.MusicBrainzArtistID)
	if mbid != "" && nfoMBID != "" && !strings.EqualFold(nfoMBID, mbid) {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  %s carries MBID %s, the artist has %s", nfoPath, nfoMBID, mbid))
		return false
	}
	return true
}

// illegalFolderChars are the characters a folder name cannot or should not
// contain on Windows and SMB shares; taggers and download clients drop or
// replace them when they create an artist folder.
const illegalFolderChars = `/\:*?"<>|`

// illegalReplacements are the usual stand-ins for an illegal character, tried
// in this order (after plain removal).
var illegalReplacements = []string{"-", "_", "+", " "}

// lidarrReplacements is Lidarr's CleanFileName table with "Replace Illegal
// Characters" on: ? to !, * to -, / and \ to +, the rest deleted. The colon is
// handled by lidarrSpelling first.
var lidarrReplacements = map[rune]string{'?': "!", '*': "-", '/': "+", '\\': "+", '<': "", '>': "", '|': "", '"': ""}

// asciiFold maps letters that have no canonical decomposition to their usual
// ASCII spelling.
var asciiFold = map[rune]string{
	'ß': "ss", 'ẞ': "SS",
	'æ': "ae", 'Æ': "AE",
	'œ': "oe", 'Œ': "OE",
	'ø': "o", 'Ø': "O",
	'đ': "d", 'Đ': "D",
	'ð': "d", 'Ð': "D",
	'þ': "th", 'Þ': "Th",
	'ł': "l", 'Ł': "L",
	'ı': "i",
}

// nameVariants lists the spellings a folder for artistName may have on a
// filesystem, without the original: trailing dots and spaces removed (Windows
// forbids them, "Boney M." becomes "Boney M"), leading dots and spaces removed
// ("...And You Will Know Us by the Trail of Dead"), illegal characters removed
// or replaced ("AC/DC" becomes "ACDC" or "AC-DC", "Doom:vs" becomes "Doom-vs"),
// Lidarr's own replacement table ("Therapy?" becomes "Therapy!"), diacritics
// stripped ("Mötley Crüe" becomes "Motley Crue"), and the combinations of these.
// Least invasive variants come first; callers compare case-insensitively, so no
// case variants are produced.
func nameVariants(artistName string) []string {
	var variants []string
	seen := map[string]bool{strings.ToLower(norm.NFC.String(artistName)): true}
	add := func(candidate string) {
		candidate = strings.TrimRight(strings.TrimSpace(candidate), ". ")
		key := strings.ToLower(norm.NFC.String(candidate))
		if candidate == "" || seen[key] {
			return
		}
		seen[key] = true
		variants = append(variants, candidate)
	}

	for _, base := range []string{artistName, stripDiacritics(artistName)} {
		add(base)
		add(strings.TrimLeft(base, ". "))
		add(removeChars(base, illegalFolderChars))
		add(lidarrSpelling(base))
		for _, replacement := range illegalReplacements {
			add(replaceChars(base, illegalFolderChars, replacement))
		}
		add(collapseSpaces(replaceChars(base, illegalFolderChars, " ")))
	}
	return variants
}

// lidarrSpelling is the artist folder name Lidarr builds with its default
// naming settings ("{Artist Name}", "Replace Illegal Characters" on, colon
// replacement "Smart"): FileNameBuilder.CleanFileName turns ": " into " - " and
// ":" into "-" and applies the lidarrReplacements table, then CleanFolderName
// squeezes a run of one separator (space, "-", ".", "_") to a single one and
// trims spaces and dots at both ends.
func lidarrSpelling(s string) string {
	s = strings.ReplaceAll(s, ": ", " - ")
	s = strings.ReplaceAll(s, ":", "-")
	return strings.Trim(collapseSeparatorRuns(mapRunes(s, lidarrReplacements)), " .")
}

// mapRunes replaces every rune found in table with its entry (an empty entry
// deletes the rune) and keeps the others.
func mapRunes(s string, table map[rune]string) string {
	var b strings.Builder
	for _, r := range s {
		if replacement, ok := table[r]; ok {
			b.WriteString(replacement)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// collapseSeparatorRuns keeps one of every run of the same separator rune
// (space, "-", ".", "_"), like Lidarr's FileNameCleanupRegex.
func collapseSeparatorRuns(s string) string {
	var b strings.Builder
	var last rune = -1
	for _, r := range s {
		if r == last && strings.ContainsRune("- ._", r) {
			continue
		}
		b.WriteRune(r)
		last = r
	}
	return b.String()
}

// stripDiacritics removes combining marks after canonical decomposition and
// folds the letters in asciiFold, so "Mötley Crüe" becomes "Motley Crue" and
// "Sólstafir" becomes "Solstafir". The result is NFC again, so a name without
// marks comes back unchanged.
func stripDiacritics(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if folded, ok := asciiFold[r]; ok {
			b.WriteString(folded)
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}

// removeChars deletes every rune of chars from s.
func removeChars(s, chars string) string {
	return replaceChars(s, chars, "")
}

// replaceChars replaces every rune of chars in s with replacement.
func replaceChars(s, chars, replacement string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(chars, r) {
			b.WriteString(replacement)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// collapseSpaces squeezes runs of whitespace into one space and trims the ends.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
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
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  failed decoding artist.nfo using %s: %v", d.name, derr))
			continue
		}
		if err := xml.Unmarshal(decoded, &a); err == nil {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  parsed artist.nfo using %s decoder", d.name))
			return a, nil
		} else {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  xml unmarshal failed after %s decode: %v", d.name, err))
		}
	}

	// Final fallback: convert bytes to string (invalid UTF-8 will be replaced)
	// and try again. This may lose/replace invalid runes but often succeeds.
	if err := xml.Unmarshal([]byte(string(data)), &a); err == nil {
		pdk.Log(pdk.LogDebug, "  parsed artist.nfo using fallback string conversion")
		return a, nil
	} else {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("  xml unmarshal failed after fallback string conversion: %v", err))
	}

	return a, fmt.Errorf("  xml unmarshal failed for artist.nfo (attempted UTF-8, windows-1252, iso-8859-1, and fallback)")
}

// loadSubpathConfig reads pdk config key "subpaths" and returns a map[libraryId]subpath.
func loadSubpathConfig() map[int]string {
	result := make(map[int]string)

	cfgStr, ok := pdk.GetConfig("subpaths")
	if !ok || strings.TrimSpace(cfgStr) == "" {
		pdk.Log(pdk.LogDebug, "  no subpaths config found (key 'subpaths' empty/missing)")
		return result
	}

	var entries []subpathConfigEntry
	if err := json.Unmarshal([]byte(cfgStr), &entries); err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("  failed to parse subpaths config: %v", err))
		return result
	}

	for _, e := range entries {
		if e.LibraryId == 0 {
			pdk.Log(pdk.LogDebug, fmt.Sprintf("  ignoring subpaths entry with libraryId=0 (subpath=%q)", e.Subpath))
			continue
		}
		result[e.LibraryId] = strings.Trim(e.Subpath, "/")
	}
	return result
}

func main() {}
