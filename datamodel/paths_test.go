package datamodel

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fixtureParams struct {
	MCL int `json:"mcl"`
	MCC int `json:"mcc"`
	MPL int `json:"mpl"`
}

type absoluteCase struct {
	Name          string   `json:"name"`
	ComponentsHex []string `json:"components_hex"`
	EncodedHex    string   `json:"encoded_hex"`
}

type absoluteFile struct {
	Params fixtureParams  `json:"params"`
	Cases  []absoluteCase `json:"cases"`
}

type relativeCase struct {
	Name                   string   `json:"name"`
	ReferenceComponentsHex []string `json:"reference_components_hex"`
	TargetComponentsHex    []string `json:"target_components_hex"`
	EncodedHex             string   `json:"encoded_hex"`
}

type relativeFile struct {
	Params fixtureParams  `json:"params"`
	Cases  []relativeCase `json:"cases"`
}

func loadAbsolute(t *testing.T, name string) absoluteFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "paths", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var f absoluteFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	return f
}

func loadRelative(t *testing.T, name string) relativeFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "paths", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var f relativeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	return f
}

func decodeHexComponents(t *testing.T, items []string) [][]byte {
	t.Helper()
	out := make([][]byte, len(items))
	for i, s := range items {
		b, err := hex.DecodeString(s)
		if err != nil {
			t.Fatalf("decode hex component %q: %v", s, err)
		}
		out[i] = b
	}
	return out
}

func limitsFromParams(p fixtureParams) Limits {
	return Limits{
		MaxComponentLength: p.MCL,
		MaxComponentCount:  p.MCC,
		MaxPathLength:      p.MPL,
	}
}

func TestPath_AbsoluteFixtures(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"basic.json", "limits.json"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file := loadAbsolute(t, name)
			limits := limitsFromParams(file.Params)

			for _, c := range file.Cases {
				c := c
				t.Run(c.Name, func(t *testing.T) {
					t.Parallel()
					comps := decodeHexComponents(t, c.ComponentsHex)
					want, err := hex.DecodeString(c.EncodedHex)
					if err != nil {
						t.Fatalf("decode expected hex: %v", err)
					}

					path, err := FromSlices(limits, comps)
					if err != nil {
						t.Fatalf("FromSlices: %v", err)
					}
					got := path.Encode()
					if !bytes.Equal(got, want) {
						t.Errorf("Encode mismatch\n got: %x\nwant: %x", got, want)
					}

					decoded, n, err := Decode(limits, want)
					if err != nil {
						t.Fatalf("Decode: %v", err)
					}
					if n != len(want) {
						t.Errorf("Decode consumed %d bytes, want %d", n, len(want))
					}
					if !decoded.Equal(path) {
						t.Errorf("Decode produced non-equal path: got components=%x want components=%x",
							componentDump(decoded), componentDump(path))
					}
				})
			}
		})
	}
}

func TestPath_RelativeFixtures(t *testing.T) {
	t.Parallel()
	file := loadRelative(t, "relative.json")
	limits := limitsFromParams(file.Params)

	for _, c := range file.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			refComps := decodeHexComponents(t, c.ReferenceComponentsHex)
			targetComps := decodeHexComponents(t, c.TargetComponentsHex)
			want, err := hex.DecodeString(c.EncodedHex)
			if err != nil {
				t.Fatalf("decode expected hex: %v", err)
			}

			ref, err := FromSlices(limits, refComps)
			if err != nil {
				t.Fatalf("FromSlices(ref): %v", err)
			}
			target, err := FromSlices(limits, targetComps)
			if err != nil {
				t.Fatalf("FromSlices(target): %v", err)
			}

			got := target.EncodeRelativeTo(ref)
			if !bytes.Equal(got, want) {
				t.Errorf("EncodeRelativeTo mismatch\n got: %x\nwant: %x", got, want)
			}

			decoded, n, err := DecodeRelative(limits, ref, want)
			if err != nil {
				t.Fatalf("DecodeRelative: %v", err)
			}
			if n != len(want) {
				t.Errorf("DecodeRelative consumed %d bytes, want %d", n, len(want))
			}
			if !decoded.Equal(target) {
				t.Errorf("DecodeRelative produced non-equal path:\n got %x\nwant %x",
					componentDump(decoded), componentDump(target))
			}
		})
	}
}

func TestPath_ConstructorErrors(t *testing.T) {
	t.Parallel()
	// Tight limits chosen so each error is exercised in isolation.
	tooLongComp := bytes.Repeat([]byte("x"), 5)
	cases := []struct {
		name   string
		limits Limits
		comps  [][]byte
		want   error
	}{
		{"component_too_long", Limits{MaxComponentLength: 4, MaxComponentCount: 4, MaxPathLength: 64}, [][]byte{tooLongComp}, ErrComponentTooLong},
		{"too_many_components", Limits{MaxComponentLength: 4, MaxComponentCount: 2, MaxPathLength: 64}, [][]byte{{1}, {2}, {3}}, ErrTooManyComponents},
		{"path_too_long", Limits{MaxComponentLength: 8, MaxComponentCount: 4, MaxPathLength: 4}, [][]byte{{1, 2, 3}, {4, 5, 6}}, ErrPathTooLong},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := FromSlices(tc.limits, tc.comps)
			if err != tc.want {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPath_LongestCommonPrefix_IsPrefixOf(t *testing.T) {
	t.Parallel()
	limits := Limits{MaxComponentLength: 16, MaxComponentCount: 16, MaxPathLength: 64}
	a := mustPath(t, limits, [][]byte{[]byte("alpha"), []byte("beta"), []byte("gamma")})
	b := mustPath(t, limits, [][]byte{[]byte("alpha"), []byte("beta"), []byte("delta")})
	c := mustPath(t, limits, [][]byte{[]byte("alpha"), []byte("beta")})

	lcp := a.LongestCommonPrefix(b)
	if !lcp.Equal(c) {
		t.Errorf("lcp(a,b) components = %x, want %x", componentDump(lcp), componentDump(c))
	}
	if !c.IsPrefixOf(a) {
		t.Error("c should be prefix of a")
	}
	if a.IsPrefixOf(b) {
		t.Error("a should not be prefix of b")
	}
}

func mustPath(t *testing.T, limits Limits, comps [][]byte) Path {
	t.Helper()
	p, err := FromSlices(limits, comps)
	if err != nil {
		t.Fatalf("FromSlices: %v", err)
	}
	return p
}

func componentDump(p Path) [][]byte { return p.Components() }

type extendsCase struct {
	Name                 string   `json:"name"`
	PrefixComponentsHex  []string `json:"prefix_components_hex"`
	PathComponentsHex    []string `json:"path_components_hex"`
	EncodedHex           string   `json:"encoded_hex"`
}

type extendsFile struct {
	Params fixtureParams  `json:"params"`
	Cases  []extendsCase  `json:"cases"`
}

func TestPath_ExtendsFixtures(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "paths_rel", "extends.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f extendsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	limits := limitsFromParams(f.Params)

	for _, c := range f.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			prefixComps := decodeHexComponents(t, c.PrefixComponentsHex)
			pathComps := decodeHexComponents(t, c.PathComponentsHex)
			want, err := hex.DecodeString(c.EncodedHex)
			if err != nil {
				t.Fatalf("decode hex: %v", err)
			}

			prefix, err := FromSlices(limits, prefixComps)
			if err != nil {
				t.Fatalf("FromSlices(prefix): %v", err)
			}
			path, err := FromSlices(limits, pathComps)
			if err != nil {
				t.Fatalf("FromSlices(path): %v", err)
			}

			got := path.EncodeExtending(prefix)
			if !bytes.Equal(got, want) {
				t.Errorf("EncodeExtending mismatch\n got: %x\nwant: %x", got, want)
			}

			decoded, n, err := DecodeExtending(prefix, want)
			if err != nil {
				t.Fatalf("DecodeExtending: %v", err)
			}
			if n != len(want) {
				t.Errorf("consumed %d bytes, want %d", n, len(want))
			}
			if !decoded.Equal(path) {
				t.Errorf("DecodeExtending got %x, want %x",
					componentDump(decoded), componentDump(path))
			}
		})
	}
}

func TestPath_EncodeExtending_PanicsOnNonPrefix(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when prefix is not a prefix of path")
		}
	}()
	prefix := pathOf(t, "other")
	path := pathOf(t, "alpha", "beta")
	_ = path.EncodeExtending(prefix)
}
