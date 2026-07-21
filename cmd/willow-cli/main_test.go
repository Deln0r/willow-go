package main

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Deln0r/willow-go/willow25"
)

// runCLI drives the testable core and returns (exitCode, stdout, stderr).
func runCLI(args ...string) (int, string, string) {
	var out, errw bytes.Buffer
	code := run(args, &out, &errw)
	return code, out.String(), errw.String()
}

func TestPathEncodeDecodeRoundTrip(t *testing.T) {
	code, enc, errs := runCLI("path", "encode", "notes", "greeting.txt")
	if code != 0 {
		t.Fatalf("encode exit %d, stderr=%q", code, errs)
	}
	encHex := strings.TrimSpace(enc)
	if encHex == "" {
		t.Fatal("empty encoding")
	}

	code, dec, errs := runCLI("path", "decode", encHex)
	if code != 0 {
		t.Fatalf("decode exit %d, stderr=%q", code, errs)
	}
	for _, want := range []string{"components:   2", `"notes"`, `"greeting.txt"`} {
		if !strings.Contains(dec, want) {
			t.Errorf("decode output missing %q:\n%s", want, dec)
		}
	}
}

func TestPathEncodeHexComponents(t *testing.T) {
	code, out, errs := runCLI("path", "encode", "-hex", "6162", "00ff")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, errs)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("empty output")
	}
}

func TestPathDecodeToleratesPrefixAndSpaces(t *testing.T) {
	// Same bytes as "notes"/"greeting.txt", with a 0x prefix and spaces.
	_, enc, _ := runCLI("path", "encode", "notes", "greeting.txt")
	encHex := strings.TrimSpace(enc)
	spaced := "0x" + encHex[:6] + " " + encHex[6:]

	code, dec, errs := runCLI("path", "decode", spaced)
	if code != 0 {
		t.Fatalf("decode of prefixed/spaced hex exit %d, stderr=%q", code, errs)
	}
	if !strings.Contains(dec, "components:   2") {
		t.Errorf("expected 2 components, got:\n%s", dec)
	}
}

func TestPathDecodeOddHexIsError(t *testing.T) {
	code, _, errs := runCLI("path", "decode", "abc")
	if code != 1 {
		t.Fatalf("odd hex should exit 1, got %d", code)
	}
	if !strings.Contains(errs, "odd number") {
		t.Errorf("expected odd-length error, got %q", errs)
	}
}

func TestEntryDecode(t *testing.T) {
	ns := bytes.Repeat([]byte{0x11}, 32)
	sub := bytes.Repeat([]byte{0x22}, 32)
	path, err := willow25.NewPath([][]byte{[]byte("alfie")})
	if err != nil {
		t.Fatal(err)
	}
	e, err := willow25.NewEntry(ns, sub, path, 12345, []byte("hello, willow"))
	if err != nil {
		t.Fatal(err)
	}
	encHex := hex.EncodeToString(e.Encode())

	code, out, errs := runCLI("entry", "decode", encHex)
	if code != 0 {
		t.Fatalf("entry decode exit %d, stderr=%q", code, errs)
	}
	for _, want := range []string{
		"namespace_id:   " + strings.Repeat("11", 32),
		"timestamp:      12345",
		"payload_length: 13",
		`"alfie"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("entry output missing %q:\n%s", want, out)
		}
	}
}

func TestDigestFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "payload.bin")
	content := []byte("abc")
	if err := os.WriteFile(f, content, 0o644); err != nil {
		t.Fatal(err)
	}
	want := willow25.William3Sum(content)

	code, out, errs := runCLI("digest", f)
	if code != 0 {
		t.Fatalf("digest exit %d, stderr=%q", code, errs)
	}
	if got := strings.TrimSpace(out); got != hex.EncodeToString(want[:]) {
		t.Errorf("digest mismatch:\n got %s\nwant %s", got, hex.EncodeToString(want[:]))
	}
}

func TestUsageAndErrors(t *testing.T) {
	if code, _, _ := runCLI(); code != 2 {
		t.Errorf("no args should exit 2, got %d", code)
	}
	if code, out, _ := runCLI("help"); code != 0 || !strings.Contains(out, "willow-cli") {
		t.Errorf("help should exit 0 and print usage, got code %d", code)
	}
	if code, _, errs := runCLI("frobnicate"); code != 2 || !strings.Contains(errs, "unknown command") {
		t.Errorf("unknown command should exit 2 with message, got code %d err %q", code, errs)
	}
	if code, _, errs := runCLI("path"); code != 1 || !strings.Contains(errs, "encode|decode") {
		t.Errorf("bare path should exit 1 with subcommand hint, got code %d err %q", code, errs)
	}
}
