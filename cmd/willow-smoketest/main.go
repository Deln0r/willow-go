// willow-smoketest is a CLI that exercises the Go impl against every
// committed byte-compat fixture under testdata/ and reports pass/fail
// counts per fixture group. Exit code 0 if all fixtures pass, 1 otherwise.
//
// This is the pre-MVP acceptance gate: every encoder we ship must produce
// byte-identical output to the willow_rs reference for every committed
// input, and every decoder must round-trip without loss.
//
// Usage:
//
//	willow-smoketest [-testdata <dir>] [-v]
//
// Defaults: -testdata=./testdata, -v=false (quiet on per-case pass).
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Deln0r/willow-go/datamodel"
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

type extendsCase struct {
	Name                string   `json:"name"`
	PrefixComponentsHex []string `json:"prefix_components_hex"`
	PathComponentsHex   []string `json:"path_components_hex"`
	EncodedHex          string   `json:"encoded_hex"`
}

type extendsFile struct {
	Params fixtureParams `json:"params"`
	Cases  []extendsCase `json:"cases"`
}

type entryParams struct {
	MCL                int `json:"mcl"`
	MCC                int `json:"mcc"`
	MPL                int `json:"mpl"`
	NamespaceIDWidth   int `json:"namespace_id_width"`
	SubspaceIDWidth    int `json:"subspace_id_width"`
	PayloadDigestWidth int `json:"payload_digest_width"`
}

type entryCase struct {
	Name              string   `json:"name"`
	NamespaceIDHex    string   `json:"namespace_id_hex"`
	SubspaceIDHex     string   `json:"subspace_id_hex"`
	PathComponentsHex []string `json:"path_components_hex"`
	Timestamp         uint64   `json:"timestamp"`
	PayloadLength     uint64   `json:"payload_length"`
	PayloadDigestHex  string   `json:"payload_digest_hex"`
	EncodedHex        string   `json:"encoded_hex"`
}

type entryFile struct {
	Params entryParams `json:"params"`
	Cases  []entryCase `json:"cases"`
}

type areaJSON struct {
	SubspaceHex       *string  `json:"subspace_hex"`
	PathComponentsHex []string `json:"path_components_hex"`
	TimesStart        uint64   `json:"times_start"`
	TimesEnd          *uint64  `json:"times_end"`
}

type areaRelativeCase struct {
	Name       string   `json:"name"`
	Rel        areaJSON `json:"rel"`
	Target     areaJSON `json:"target"`
	EncodedHex string   `json:"encoded_hex"`
}

type areaRelativeFile struct {
	Params struct {
		MCL             int `json:"mcl"`
		MCC             int `json:"mcc"`
		MPL             int `json:"mpl"`
		SubspaceIDWidth int `json:"subspace_id_width"`
	} `json:"params"`
	Cases []areaRelativeCase `json:"cases"`
}

func main() {
	testdataDir := flag.String("testdata", "testdata", "path to the testdata directory")
	verbose := flag.Bool("v", false, "log each fixture case as it runs")
	flag.Parse()

	var totalPass, totalFail int
	var failures []string

	groups := []struct {
		name string
		run  func(dir string, verbose bool) (pass, fail int, fails []string)
	}{
		{"paths (absolute)", runAbsolutePaths},
		{"paths (relative)", runRelativePaths},
		{"paths (extends)", runExtendsPaths},
		{"entries", runEntries},
		{"areas (relative)", runAreaRelative},
	}

	for _, g := range groups {
		p, f, fails := g.run(*testdataDir, *verbose)
		fmt.Printf("%-20s  %3d pass  %3d fail\n", g.name, p, f)
		totalPass += p
		totalFail += f
		failures = append(failures, fails...)
	}

	fmt.Printf("\nTOTAL: %d pass / %d fail (%d cases)\n", totalPass, totalFail, totalPass+totalFail)
	if totalFail > 0 {
		fmt.Fprintf(os.Stderr, "\n%d failures:\n", totalFail)
		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "  -", f)
		}
		os.Exit(1)
	}
}

func runAbsolutePaths(dir string, verbose bool) (pass, fail int, failures []string) {
	for _, name := range []string{"basic.json", "limits.json"} {
		path := filepath.Join(dir, "paths", name)
		raw, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("read %s: %v", path, err))
			fail++
			continue
		}
		var f absoluteFile
		if err := json.Unmarshal(raw, &f); err != nil {
			failures = append(failures, fmt.Sprintf("unmarshal %s: %v", path, err))
			fail++
			continue
		}
		limits := limitsFromFixture(f.Params)
		for _, c := range f.Cases {
			label := fmt.Sprintf("paths/%s::%s", name, c.Name)
			comps := decodeHexAll(c.ComponentsHex)
			p, err := datamodel.FromSlices(limits, comps)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: FromSlices: %v", label, err))
				fail++
				continue
			}
			want, _ := hex.DecodeString(c.EncodedHex)
			got := p.Encode()
			if !bytes.Equal(got, want) {
				failures = append(failures, fmt.Sprintf("%s: encode mismatch", label))
				fail++
				continue
			}
			dec, n, err := datamodel.Decode(limits, want)
			if err != nil || n != len(want) || !dec.Equal(p) {
				failures = append(failures, fmt.Sprintf("%s: decode round-trip failed", label))
				fail++
				continue
			}
			if verbose {
				fmt.Printf("  PASS %s\n", label)
			}
			pass++
		}
	}
	return
}

func runRelativePaths(dir string, verbose bool) (pass, fail int, failures []string) {
	path := filepath.Join(dir, "paths", "relative.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read %s: %v", path, err))
		return 0, 1, failures
	}
	var f relativeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		failures = append(failures, fmt.Sprintf("unmarshal: %v", err))
		return 0, 1, failures
	}
	limits := limitsFromFixture(f.Params)
	for _, c := range f.Cases {
		label := fmt.Sprintf("paths/relative.json::%s", c.Name)
		refComps := decodeHexAll(c.ReferenceComponentsHex)
		targetComps := decodeHexAll(c.TargetComponentsHex)
		ref, err := datamodel.FromSlices(limits, refComps)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: ref FromSlices: %v", label, err))
			fail++
			continue
		}
		target, err := datamodel.FromSlices(limits, targetComps)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: target FromSlices: %v", label, err))
			fail++
			continue
		}
		want, _ := hex.DecodeString(c.EncodedHex)
		got := target.EncodeRelativeTo(ref)
		if !bytes.Equal(got, want) {
			failures = append(failures, fmt.Sprintf("%s: encode mismatch", label))
			fail++
			continue
		}
		dec, n, err := datamodel.DecodeRelative(limits, ref, want)
		if err != nil || n != len(want) || !dec.Equal(target) {
			failures = append(failures, fmt.Sprintf("%s: decode round-trip failed", label))
			fail++
			continue
		}
		if verbose {
			fmt.Printf("  PASS %s\n", label)
		}
		pass++
	}
	return
}

func runExtendsPaths(dir string, verbose bool) (pass, fail int, failures []string) {
	path := filepath.Join(dir, "paths_rel", "extends.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read %s: %v", path, err))
		return 0, 1, failures
	}
	var f extendsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		failures = append(failures, fmt.Sprintf("unmarshal: %v", err))
		return 0, 1, failures
	}
	limits := limitsFromFixture(f.Params)
	for _, c := range f.Cases {
		label := fmt.Sprintf("paths_rel/extends.json::%s", c.Name)
		prefixComps := decodeHexAll(c.PrefixComponentsHex)
		pathComps := decodeHexAll(c.PathComponentsHex)
		prefix, _ := datamodel.FromSlices(limits, prefixComps)
		fullPath, _ := datamodel.FromSlices(limits, pathComps)
		want, _ := hex.DecodeString(c.EncodedHex)
		got := fullPath.EncodeExtending(prefix)
		if !bytes.Equal(got, want) {
			failures = append(failures, fmt.Sprintf("%s: encode mismatch", label))
			fail++
			continue
		}
		dec, n, err := datamodel.DecodeExtending(prefix, want)
		if err != nil || n != len(want) || !dec.Equal(fullPath) {
			failures = append(failures, fmt.Sprintf("%s: decode round-trip failed", label))
			fail++
			continue
		}
		if verbose {
			fmt.Printf("  PASS %s\n", label)
		}
		pass++
	}
	return
}

func runEntries(dir string, verbose bool) (pass, fail int, failures []string) {
	path := filepath.Join(dir, "entries", "basic.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read %s: %v", path, err))
		return 0, 1, failures
	}
	var f entryFile
	if err := json.Unmarshal(raw, &f); err != nil {
		failures = append(failures, fmt.Sprintf("unmarshal: %v", err))
		return 0, 1, failures
	}
	spec := datamodel.EntrySpec{
		Limits: datamodel.Limits{
			MaxComponentLength: f.Params.MCL,
			MaxComponentCount:  f.Params.MCC,
			MaxPathLength:      f.Params.MPL,
		},
		NamespaceIDLength:   f.Params.NamespaceIDWidth,
		SubspaceIDLength:    f.Params.SubspaceIDWidth,
		PayloadDigestLength: f.Params.PayloadDigestWidth,
	}
	for _, c := range f.Cases {
		label := fmt.Sprintf("entries/basic.json::%s", c.Name)
		comps := decodeHexAll(c.PathComponentsHex)
		p, err := datamodel.FromSlices(spec.Limits, comps)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: path FromSlices: %v", label, err))
			fail++
			continue
		}
		ns, _ := hex.DecodeString(c.NamespaceIDHex)
		sub, _ := hex.DecodeString(c.SubspaceIDHex)
		digest, _ := hex.DecodeString(c.PayloadDigestHex)
		entry := datamodel.Entry{
			NamespaceID:   ns,
			SubspaceID:    sub,
			Path:          p,
			Timestamp:     c.Timestamp,
			PayloadLength: c.PayloadLength,
			PayloadDigest: digest,
		}
		want, _ := hex.DecodeString(c.EncodedHex)
		got := entry.Encode()
		if !bytes.Equal(got, want) {
			failures = append(failures, fmt.Sprintf("%s: encode mismatch", label))
			fail++
			continue
		}
		dec, n, err := datamodel.DecodeEntry(spec, want)
		if err != nil || n != len(want) {
			failures = append(failures, fmt.Sprintf("%s: decode failed: err=%v consumed=%d/%d", label, err, n, len(want)))
			fail++
			continue
		}
		if !bytes.Equal(dec.NamespaceID, entry.NamespaceID) ||
			!bytes.Equal(dec.SubspaceID, entry.SubspaceID) ||
			!dec.Path.Equal(entry.Path) ||
			dec.Timestamp != entry.Timestamp ||
			dec.PayloadLength != entry.PayloadLength ||
			!bytes.Equal(dec.PayloadDigest, entry.PayloadDigest) {
			failures = append(failures, fmt.Sprintf("%s: decode field mismatch", label))
			fail++
			continue
		}
		if verbose {
			fmt.Printf("  PASS %s\n", label)
		}
		pass++
	}
	return
}

func runAreaRelative(dir string, verbose bool) (pass, fail int, failures []string) {
	path := filepath.Join(dir, "areas", "relative.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read %s: %v", path, err))
		return 0, 1, failures
	}
	var f areaRelativeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		failures = append(failures, fmt.Sprintf("unmarshal: %v", err))
		return 0, 1, failures
	}
	limits := datamodel.Limits{
		MaxComponentLength: f.Params.MCL,
		MaxComponentCount:  f.Params.MCC,
		MaxPathLength:      f.Params.MPL,
	}
	for _, c := range f.Cases {
		label := fmt.Sprintf("areas/relative.json::%s", c.Name)
		rel, err := areaFromJSON(limits, c.Rel)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: rel: %v", label, err))
			fail++
			continue
		}
		target, err := areaFromJSON(limits, c.Target)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: target: %v", label, err))
			fail++
			continue
		}
		want, _ := hex.DecodeString(c.EncodedHex)
		got := target.EncodeRelativeTo(rel)
		if !bytes.Equal(got, want) {
			failures = append(failures, fmt.Sprintf("%s: encode mismatch", label))
			fail++
			continue
		}
		dec, n, err := datamodel.DecodeAreaRelativeTo(limits, rel, f.Params.SubspaceIDWidth, want)
		if err != nil || n != len(want) || !areaEqual(dec, target) {
			failures = append(failures, fmt.Sprintf("%s: decode round-trip failed: err=%v", label, err))
			fail++
			continue
		}
		if verbose {
			fmt.Printf("  PASS %s\n", label)
		}
		pass++
	}
	return
}

func areaFromJSON(limits datamodel.Limits, j areaJSON) (datamodel.Area, error) {
	var sub *[]byte
	if j.SubspaceHex != nil {
		b, err := hex.DecodeString(*j.SubspaceHex)
		if err != nil {
			return datamodel.Area{}, fmt.Errorf("subspace hex: %w", err)
		}
		sub = &b
	}
	comps := decodeHexAll(j.PathComponentsHex)
	p, err := datamodel.FromSlices(limits, comps)
	if err != nil {
		return datamodel.Area{}, err
	}
	var times datamodel.TimeRange
	if j.TimesEnd == nil {
		times = datamodel.NewTimeRangeOpen(j.TimesStart)
	} else {
		times, err = datamodel.NewTimeRangeClosed(j.TimesStart, *j.TimesEnd)
		if err != nil {
			return datamodel.Area{}, err
		}
	}
	return datamodel.Area{Subspace: sub, PathPrefix: p, Times: times}, nil
}

func areaEqual(a, b datamodel.Area) bool {
	if (a.Subspace == nil) != (b.Subspace == nil) {
		return false
	}
	if a.Subspace != nil && !bytes.Equal(*a.Subspace, *b.Subspace) {
		return false
	}
	if !a.PathPrefix.Equal(b.PathPrefix) {
		return false
	}
	return a.Times == b.Times
}

func decodeHexAll(items []string) [][]byte {
	out := make([][]byte, len(items))
	for i, s := range items {
		b, _ := hex.DecodeString(s)
		out[i] = b
	}
	return out
}

func limitsFromFixture(p fixtureParams) datamodel.Limits {
	return datamodel.Limits{
		MaxComponentLength: p.MCL,
		MaxComponentCount:  p.MCC,
		MaxPathLength:      p.MPL,
	}
}
