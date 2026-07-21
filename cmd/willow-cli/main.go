// willow-cli is a read-only inspection and debugging tool for Willow
// encodings. It encodes and decodes the canonical wire formats that
// willow-go ships today, which makes it useful for cross-implementation
// interop debugging: paste a hex dump from willow_rs or willow-js and see
// how this implementation parses it, or produce a canonical encoding to
// compare byte-for-byte against another implementation.
//
// It uses the Willow'25 parameter bundle (4096/4096/4096, 32-byte ids).
//
// Subcommands:
//
//	path encode [-hex] <component>...   canonical Path encoding, as hex
//	path decode <hex>                   decode a Path encoding
//	entry decode <hex>                  decode an Entry encoding
//	digest [file]                       WILLIAM3 digest of a file (or stdin)
//
// Hex arguments tolerate a leading 0x and internal whitespace, so a hex dump
// copied from another tool can be pasted as-is.
//
// It does NOT cover capability encodings: Willow has no settled canonical
// wire format for Meadowcap capabilities yet (encode_mc_capability is Phase 2,
// see TECH_DEBT.md), so there is nothing stable to inspect.
//
// Everything here is local and read-only. There is no network, no sync, and
// no key generation.
package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Deln0r/willow-go/datamodel"
	"github.com/Deln0r/willow-go/willow25"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one CLI invocation and returns the process exit code. It is the
// testable core: tests drive it with argument slices and capture out/errw.
func run(args []string, out, errw io.Writer) int {
	if len(args) < 1 {
		usage(errw)
		return 2
	}
	var err error
	switch args[0] {
	case "path":
		err = runPath(args[1:], out)
	case "entry":
		err = runEntry(args[1:], out)
	case "digest":
		err = runDigest(args[1:], out)
	case "-h", "--help", "help":
		usage(out)
		return 0
	default:
		fmt.Fprintf(errw, "unknown command %q\n\n", args[0])
		usage(errw)
		return 2
	}
	if err != nil {
		fmt.Fprintf(errw, "error: %v\n", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `willow-cli is a read-only inspector for Willow'25 encodings.

usage:
  willow-cli path encode [-hex] <component>...   canonical Path encoding, as hex
  willow-cli path decode <hex>                   decode a Path encoding
  willow-cli entry decode <hex>                  decode an Entry encoding
  willow-cli digest [file]                       WILLIAM3 digest of a file (stdin if omitted)

notes:
  -hex   treat each path component as a hex string instead of literal bytes
  Hex arguments accept a leading 0x and internal whitespace.
  Capability encodings are not covered: Willow has no canonical capability
  wire format yet (Phase 2, see TECH_DEBT.md).
`)
}

func runPath(args []string, out io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("path: need a subcommand (encode|decode)")
	}
	switch args[0] {
	case "encode":
		return runPathEncode(args[1:], out)
	case "decode":
		return runPathDecode(args[1:], out)
	default:
		return fmt.Errorf("path: unknown subcommand %q (want encode|decode)", args[0])
	}
}

func runPathEncode(args []string, out io.Writer) error {
	asHex := false
	if len(args) > 0 && args[0] == "-hex" {
		asHex = true
		args = args[1:]
	}
	comps := make([][]byte, 0, len(args))
	for i, a := range args {
		if asHex {
			b, err := decodeHex(a)
			if err != nil {
				return fmt.Errorf("component %d: %w", i, err)
			}
			comps = append(comps, b)
		} else {
			comps = append(comps, []byte(a))
		}
	}
	p, err := datamodel.FromSlices(willow25.Limits(), comps)
	if err != nil {
		return fmt.Errorf("build path: %w", err)
	}
	fmt.Fprintln(out, hex.EncodeToString(p.Encode()))
	return nil
}

func runPathDecode(args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("path decode: want exactly one hex argument")
	}
	src, err := decodeHex(args[0])
	if err != nil {
		return err
	}
	p, n, err := datamodel.Decode(willow25.Limits(), src)
	if err != nil {
		return fmt.Errorf("decode path: %w", err)
	}
	fmt.Fprintf(out, "components:   %d\n", p.ComponentCount())
	fmt.Fprintf(out, "total length: %d\n", p.TotalLength())
	fmt.Fprintf(out, "consumed:     %d of %d bytes\n", n, len(src))
	if n != len(src) {
		fmt.Fprintf(out, "warning:      %d trailing byte(s) not part of the path\n", len(src)-n)
	}
	for i, c := range p.Components() {
		fmt.Fprintf(out, "  [%d] %s %s\n", i, hex.EncodeToString(c), printable(c))
	}
	return nil
}

func runEntry(args []string, out io.Writer) error {
	if len(args) < 1 || args[0] != "decode" {
		return fmt.Errorf("entry: need subcommand 'decode'")
	}
	rest := args[1:]
	if len(rest) != 1 {
		return fmt.Errorf("entry decode: want exactly one hex argument")
	}
	src, err := decodeHex(rest[0])
	if err != nil {
		return err
	}
	e, n, err := datamodel.DecodeEntry(willow25.EntrySpec(), src)
	if err != nil {
		return fmt.Errorf("decode entry: %w", err)
	}
	fmt.Fprintf(out, "namespace_id:   %s\n", hex.EncodeToString(e.NamespaceID))
	fmt.Fprintf(out, "subspace_id:    %s\n", hex.EncodeToString(e.SubspaceID))
	fmt.Fprintf(out, "path:           %d components, %d bytes\n", e.Path.ComponentCount(), e.Path.TotalLength())
	for i, c := range e.Path.Components() {
		fmt.Fprintf(out, "  [%d] %s %s\n", i, hex.EncodeToString(c), printable(c))
	}
	fmt.Fprintf(out, "timestamp:      %d\n", e.Timestamp)
	fmt.Fprintf(out, "payload_length: %d\n", e.PayloadLength)
	fmt.Fprintf(out, "payload_digest: %s\n", hex.EncodeToString(e.PayloadDigest))
	fmt.Fprintf(out, "consumed:       %d of %d bytes\n", n, len(src))
	if n != len(src) {
		fmt.Fprintf(out, "warning:        %d trailing byte(s) not part of the entry\n", len(src)-n)
	}
	return nil
}

func runDigest(args []string, out io.Writer) error {
	var data []byte
	var err error
	switch len(args) {
	case 0:
		data, err = io.ReadAll(os.Stdin)
	case 1:
		data, err = os.ReadFile(args[0])
	default:
		return fmt.Errorf("digest: want at most one file argument")
	}
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	sum := willow25.William3Sum(data)
	fmt.Fprintln(out, hex.EncodeToString(sum[:]))
	return nil
}

// decodeHex parses a hex string, tolerating a leading 0x/0X and any internal
// whitespace so a dump copied from another tool works unmodified. It reports a
// specific error for the common odd-length mistake.
func decodeHex(s string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	cleaned = strings.TrimPrefix(cleaned, "0x")
	cleaned = strings.TrimPrefix(cleaned, "0X")
	if len(cleaned)%2 != 0 {
		return nil, fmt.Errorf("hex input has an odd number of digits (%d)", len(cleaned))
	}
	b, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("input is not valid hex: %w", err)
	}
	return b, nil
}

// printable renders a quoted ASCII view of c when it is fully printable,
// otherwise the empty string. It is a display aid only.
func printable(c []byte) string {
	if len(c) == 0 {
		return `""`
	}
	for _, b := range c {
		if b < 0x20 || b > 0x7e {
			return ""
		}
	}
	return `"` + string(c) + `"`
}
