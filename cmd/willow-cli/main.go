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
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "path":
		err = runPath(os.Args[2:])
	case "entry":
		err = runEntry(os.Args[2:])
	case "digest":
		err = runDigest(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `willow-cli is a read-only inspector for Willow'25 encodings.

usage:
  willow-cli path encode [-hex] <component>...   canonical Path encoding, as hex
  willow-cli path decode <hex>                   decode a Path encoding
  willow-cli entry decode <hex>                  decode an Entry encoding
  willow-cli digest [file]                       WILLIAM3 digest of a file (stdin if omitted)

notes:
  -hex   treat each path component as a hex string instead of literal bytes
  Capability encodings are not covered: Willow has no canonical capability
  wire format yet (Phase 2, see TECH_DEBT.md).
`)
}

func runPath(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("path: need a subcommand (encode|decode)")
	}
	switch args[0] {
	case "encode":
		return runPathEncode(args[1:])
	case "decode":
		return runPathDecode(args[1:])
	default:
		return fmt.Errorf("path: unknown subcommand %q (want encode|decode)", args[0])
	}
}

func runPathEncode(args []string) error {
	asHex := false
	if len(args) > 0 && args[0] == "-hex" {
		asHex = true
		args = args[1:]
	}
	comps := make([][]byte, 0, len(args))
	for i, a := range args {
		if asHex {
			b, err := hex.DecodeString(a)
			if err != nil {
				return fmt.Errorf("component %d is not valid hex: %w", i, err)
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
	fmt.Println(hex.EncodeToString(p.Encode()))
	return nil
}

func runPathDecode(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("path decode: want exactly one hex argument")
	}
	src, err := hex.DecodeString(strings.TrimSpace(args[0]))
	if err != nil {
		return fmt.Errorf("input is not valid hex: %w", err)
	}
	p, n, err := datamodel.Decode(willow25.Limits(), src)
	if err != nil {
		return fmt.Errorf("decode path: %w", err)
	}
	fmt.Printf("components:   %d\n", p.ComponentCount())
	fmt.Printf("total length: %d\n", p.TotalLength())
	fmt.Printf("consumed:     %d of %d bytes\n", n, len(src))
	for i, c := range p.Components() {
		fmt.Printf("  [%d] %s %s\n", i, hex.EncodeToString(c), printable(c))
	}
	return nil
}

func runEntry(args []string) error {
	if len(args) < 1 || args[0] != "decode" {
		return fmt.Errorf("entry: need subcommand 'decode'")
	}
	rest := args[1:]
	if len(rest) != 1 {
		return fmt.Errorf("entry decode: want exactly one hex argument")
	}
	src, err := hex.DecodeString(strings.TrimSpace(rest[0]))
	if err != nil {
		return fmt.Errorf("input is not valid hex: %w", err)
	}
	e, n, err := datamodel.DecodeEntry(willow25.EntrySpec(), src)
	if err != nil {
		return fmt.Errorf("decode entry: %w", err)
	}
	fmt.Printf("namespace_id:   %s\n", hex.EncodeToString(e.NamespaceID))
	fmt.Printf("subspace_id:    %s\n", hex.EncodeToString(e.SubspaceID))
	fmt.Printf("path:           %d components, %d bytes\n", e.Path.ComponentCount(), e.Path.TotalLength())
	for i, c := range e.Path.Components() {
		fmt.Printf("  [%d] %s %s\n", i, hex.EncodeToString(c), printable(c))
	}
	fmt.Printf("timestamp:      %d\n", e.Timestamp)
	fmt.Printf("payload_length: %d\n", e.PayloadLength)
	fmt.Printf("payload_digest: %s\n", hex.EncodeToString(e.PayloadDigest))
	fmt.Printf("consumed:       %d of %d bytes\n", n, len(src))
	return nil
}

func runDigest(args []string) error {
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
	fmt.Println(hex.EncodeToString(sum[:]))
	return nil
}

// printable renders a quoted ASCII view of c when it is mostly printable,
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
