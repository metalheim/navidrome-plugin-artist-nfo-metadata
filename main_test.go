package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/stretchr/testify/mock"
	"golang.org/x/text/unicode/norm"
)

// allowLogging lets pdk.Log run on the host, where it is a mock that panics
// without an expectation.
func allowLogging(t *testing.T) {
	pdk.ResetMock()
	pdk.PDKMock.On("Log", mock.Anything, mock.Anything).Return()
	t.Cleanup(pdk.ResetMock)
}

func TestNameVariants(t *testing.T) {
	tests := []struct {
		name  string
		first string   // the least invasive spelling must come first
		wants []string // every spelling that must be offered
		never []string
	}{
		{name: "Metallica"},
		{name: "Boney M.", first: "Boney M", wants: []string{"Boney M"}},
		{name: "P.O.D.", first: "P.O.D", wants: []string{"P.O.D"}},
		{name: "W.A.S.P.", first: "W.A.S.P", wants: []string{"W.A.S.P"}},
		{name: "AC/DC", first: "ACDC", wants: []string{"ACDC", "AC-DC", "AC_DC", "AC+DC", "AC DC"}},
		{name: "Doom:vs", first: "Doomvs", wants: []string{"Doomvs", "Doom-vs", "Doom_vs", "Doom vs"}},
		{name: "Mötley Crüe", first: "Motley Crue", wants: []string{"Motley Crue"}},
		{name: "Motörhead", first: "Motorhead", wants: []string{"Motorhead"}},
		{name: "Blue Öyster Cult", first: "Blue Oyster Cult", wants: []string{"Blue Oyster Cult"}},
		{name: "Mø", first: "Mo", wants: []string{"Mo"}},
		{name: "Hüsker Dü", first: "Husker Du", wants: []string{"Husker Du"}},
		{name: "Therapy?", first: "Therapy", wants: []string{"Therapy", "Therapy!"}},
		{name: "...And You Will Know Us by the Trail of Dead", first: "And You Will Know Us by the Trail of Dead"},
		{name: "Artist: Subtitle", wants: []string{"Artist - Subtitle", "Artist Subtitle", "Artist- Subtitle"}},
		{name: "Sigur Rós: Ágætis", wants: []string{"Sigur Ros - Agaetis", "Sigur Ros Agaetis", "Sigur Rós - Ágætis"}},
		{name: "Who?", first: "Who", wants: []string{"Who", "Who!"}, never: []string{""}},
		{name: "...", never: []string{"", "...", ".."}},
		{name: "???", never: []string{"", "???"}},
		{name: "한글"}, // no combining marks: NFD must not produce a decomposed copy
		{name: "Dёргать!", first: "Dергать!", wants: []string{"Dергать!"}}, // the diaeresis of ё is a combining mark
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nameVariants(tc.name)
			if tc.first == "" && len(tc.wants) == 0 && tc.never == nil && len(got) != 0 {
				t.Fatalf("nameVariants(%q) = %q, want none", tc.name, got)
			}
			if tc.first != "" && (len(got) == 0 || got[0] != tc.first) {
				t.Errorf("nameVariants(%q) starts with %q, want %q (all: %q)", tc.name, first(got), tc.first, got)
			}
			for _, want := range tc.wants {
				if !slices.Contains(got, want) {
					t.Errorf("nameVariants(%q) lacks %q (all: %q)", tc.name, want, got)
				}
			}
			for _, bad := range tc.never {
				if slices.Contains(got, bad) {
					t.Errorf("nameVariants(%q) offers %q (all: %q)", tc.name, bad, got)
				}
			}
			if slices.Contains(got, tc.name) {
				t.Errorf("nameVariants(%q) repeats the original name (all: %q)", tc.name, got)
			}
			seen := map[string]bool{}
			for _, v := range got {
				if seen[v] {
					t.Errorf("nameVariants(%q) repeats %q (all: %q)", tc.name, v, got)
				}
				seen[v] = true
				if norm.NFC.String(v) != v {
					t.Errorf("nameVariants(%q) offers %q, which is not NFC", tc.name, v)
				}
			}
		})
	}
}

func first(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func TestLidarrSpelling(t *testing.T) {
	tests := map[string]string{
		"Therapy?":          "Therapy!",
		"AC/DC":             "AC+DC",
		"Doom:vs":           "Doom-vs",
		"Artist: Subtitle":  "Artist - Subtitle",
		"...And You Will":   "And You Will",
		"Boney M.":          "Boney M",
		"P.O.D.":            "P.O.D",
		`Say "Hi" <now>|*`:  "Say Hi now-",
		"a  b--c . d":       "a b-c . d",
		"Metallica":         "Metallica",
		"  leading spaces ": "leading spaces",
	}
	for in, want := range tests {
		if got := lidarrSpelling(in); got != want {
			t.Errorf("lidarrSpelling(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripDiacritics(t *testing.T) {
	tests := map[string]string{
		"Mötley Crüe":    "Motley Crue",
		"Sólstafir":      "Solstafir",
		"Björk":          "Bjork",
		"Mø":             "Mo",
		"Łukasz":         "Lukasz",
		"Straße":         "Strasse",
		"Ólafur Arnalds": "Olafur Arnalds",
		"Dёргать!":       "Dергать!", // Cyrillic io decomposes to ie + diaeresis; the Latin D stays
		"한글":             "한글",       // decomposes into jamo and must come back composed
		"Metallica":      "Metallica",
		"":               "",
	}
	for in, want := range tests {
		if got := stripDiacritics(in); got != want {
			t.Errorf("stripDiacritics(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindMatchingDir(t *testing.T) {
	allowLogging(t)
	parent := t.TempDir()
	dirs := []string{"ACDC", "AC-DC", "Boney M", "Doom-vs", "Motley Crue", "P.O.D", "W.A.S.P", "metallica", "Slayer", "Therapy!"}
	dirs = append(dirs, norm.NFD.String("Die Ärzte")) // a folder created on macOS
	for _, dir := range dirs {
		if err := os.Mkdir(filepath.Join(parent, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// a file, not a directory, must never match
	if err := os.WriteFile(filepath.Join(parent, "Iron Maiden"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		artist string
		want   string // directory name; "" means no match
	}{
		{artist: "Metallica", want: "metallica"}, // case-insensitive, as before
		{artist: "Slayer", want: "Slayer"},
		{artist: "AC/DC", want: "ACDC"}, // removal wins over replacement
		{artist: "Boney M.", want: "Boney M"},
		{artist: "Doom:vs", want: "Doom-vs"},
		{artist: "Mötley Crüe", want: "Motley Crue"},
		{artist: "P.O.D.", want: "P.O.D"},
		{artist: "W.A.S.P.", want: "W.A.S.P"},
		{artist: "Therapy?", want: "Therapy!"},
		{artist: "Die Ärzte", want: norm.NFD.String("Die Ärzte")}, // NFC name, NFD folder
		{artist: "Iron Maiden"},
		{artist: "Megadeth"},
		{artist: "Pöd"}, // "Pod" is not a directory; "P.O.D" must not be matched by accident
	}
	for _, tc := range tests {
		t.Run(tc.artist, func(t *testing.T) {
			got, ok := findMatchingDir(parent, tc.artist)
			if tc.want == "" {
				if ok {
					t.Fatalf("findMatchingDir(%q) = %q, want no match", tc.artist, got)
				}
				return
			}
			if !ok || got != filepath.Join(parent, tc.want) {
				t.Fatalf("findMatchingDir(%q) = %q, %v; want %q", tc.artist, got, ok, filepath.Join(parent, tc.want))
			}
		})
	}

	if _, ok := findMatchingDir(filepath.Join(parent, "does-not-exist"), "Metallica"); ok {
		t.Fatal("findMatchingDir on a missing parent must not match")
	}
}

// writeNFO creates <parent>/<dir>/artist.nfo with the given name and MBID (either may be empty).
func writeNFO(t *testing.T, parent, dir, name, mbid string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(parent, dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "<artist>\n"
	if name != "" {
		body += "  <name>" + name + "</name>\n"
	}
	if mbid != "" {
		body += "  <musicbrainzartistid>" + mbid + "</musicbrainzartistid>\n"
	}
	body += "  <biography>text</biography>\n</artist>\n"
	path := filepath.Join(parent, dir, "artist.nfo")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindNFO(t *testing.T) {
	allowLogging(t)
	pdk.PDKMock.On("GetConfig", "subpaths").Return("", false)
	mount := t.TempDir()
	libs := []host.Library{{ID: 1, Name: "Music", Path: "/music", MountPoint: mount}}
	host.LibraryMock.On("GetAllLibraries").Return(libs, nil)
	t.Cleanup(func() { host.LibraryMock.ExpectedCalls = nil })

	const acdc = "66c662b6-6e2f-4930-8610-912e24c63ed1"
	const boa = "11111111-1111-1111-1111-111111111111"
	exact := writeNFO(t, mount, "Metallica", "Metallica", "")
	renamed := writeNFO(t, mount, "ACDC", "AC/DC", acdc)
	nameless := writeNFO(t, mount, "Motley Crue", "", "")
	namesake := writeNFO(t, mount, "BoA", "BoA", boa)

	tests := []struct {
		artist, mbid string
		want         string // nfo path, "" for not found
	}{
		{artist: "Metallica", want: exact},
		{artist: "AC/DC", want: renamed},                                // folder spelling, nfo names the artist
		{artist: "AC/DC", mbid: acdc, want: renamed},                    // same, MBID agrees
		{artist: "AC/DC", mbid: "22222222-2222-2222-2222-222222222222"}, // MBID contradicts the nfo
		{artist: "Mötley Crüe", want: nameless},                         // nothing in the nfo contradicts
		{artist: "BoA", want: namesake},                                 // exact folder, as before
		{artist: "Bôa"},                                                 // folder spelling matches BoA, nfo says another artist
		{artist: "Megadeth"},
	}
	for _, tc := range tests {
		t.Run(tc.artist+"/"+tc.mbid, func(t *testing.T) {
			got, err := findNFO(tc.artist, tc.mbid)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("findNFO(%q, %q) = %q, want not found", tc.artist, tc.mbid, got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("findNFO(%q, %q) = %q, %v; want %q", tc.artist, tc.mbid, got, err, tc.want)
			}
		})
	}
}
