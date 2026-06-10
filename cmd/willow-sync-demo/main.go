// willow-sync-demo is a minimal proof-of-concept that two willow-go
// processes can exchange Meadowcap-authorised entries over a duplex
// transport. The transport here is just stdin / stdout (pipe one peer's
// stdout into another peer's stdin), and the wire framing is a
// length-prefixed concatenation of:
//
//	[entry_len u32be][encoded entry bytes]
//	[token_sig u8x64][token_cap_bytes_len u32be][token_cap_bytes]
//
// where token_cap_bytes is a length-prefixed self-describing serialisation
// of the CommunalCapability (namespace key, genesis user key, access mode,
// delegation chain). Each peer:
//
//  1. Generates a fresh Ed25519 root keypair (or reuses one from --seed).
//  2. Mints a CommunalCapability for itself.
//  3. Optionally adds N entries to its local InMemoryStore, each signed by
//     the corresponding receiver.
//  4. In --send mode, writes all locally-held authorised entries to stdout.
//  5. In --recv mode, reads from stdin, verifies each AuthorisationToken
//     (which requires running IsValid + capability scope check + signature
//     check), and inserts accepted entries into its store.
//
// Run two peers in opposite directions to demonstrate sync. This is NOT
// the WGPS sync protocol; that is Phase 2 (see TECH_DEBT.md). This demo
// shows the end-to-end capability layer works on wire.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Deln0r/willow-go/datamodel"
	"github.com/Deln0r/willow-go/meadowcap"
	"github.com/Deln0r/willow-go/willow25"
)

func main() {
	mode := flag.String("mode", "", "one of: gen (gen+sign N entries and send), recv (read from stdin and insert)")
	count := flag.Int("count", 3, "in gen mode: how many entries to produce")
	tag := flag.String("tag", "peer", "label printed on diagnostic lines so two peers are distinguishable")
	flag.Parse()

	switch *mode {
	case "gen":
		if err := runGen(*count, *tag, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "%s gen: %v\n", *tag, err)
			os.Exit(1)
		}
	case "recv":
		if err := runRecv(*tag, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "%s recv: %v\n", *tag, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: willow-sync-demo --mode=gen|recv [--count=N] [--tag=NAME]")
		fmt.Fprintln(os.Stderr, "example: willow-sync-demo --mode=gen --count=5 --tag=alice | willow-sync-demo --mode=recv --tag=bob")
		os.Exit(2)
	}
}

func runGen(count int, tag string, out io.Writer) error {
	nsPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	rootPub, rootPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	cap, err := meadowcap.NewCommunal(meadowcap.AccessModeWrite, nsPub, rootPub)
	if err != nil {
		return err
	}

	store := datamodel.NewInMemoryStore()

	fmt.Fprintf(os.Stderr, "%s gen: namespace=%x... root=%x...\n", tag, nsPub[:8], rootPub[:8])

	for i := 0; i < count; i++ {
		payload := []byte(fmt.Sprintf("%s payload #%d", tag, i))
		digest := willow25.HashPayload(payload)
		path, err := willow25.NewPath([][]byte{
			[]byte("notes"),
			[]byte(fmt.Sprintf("entry-%03d", i)),
		})
		if err != nil {
			return fmt.Errorf("NewPath: %w", err)
		}
		entry := datamodel.Entry{
			NamespaceID:   nsPub,
			SubspaceID:    rootPub,
			Path:          path,
			Timestamp:     uint64(100 + i*1000),
			PayloadLength: uint64(len(payload)),
			PayloadDigest: digest[:],
		}
		token, err := meadowcap.NewAuthorisationToken(cap, rootPriv, entry)
		if err != nil {
			return fmt.Errorf("AuthorisationToken: %w", err)
		}
		if _, stored := store.Insert(entry); !stored {
			return errors.New("local insert refused (bug)")
		}
		if err := writeFrame(out, entry, token); err != nil {
			return fmt.Errorf("writeFrame: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "%s gen: wrote %d entries\n", tag, count)
	return nil
}

func runRecv(tag string, in io.Reader) error {
	store := datamodel.NewInMemoryStore()
	read, accepted, rejected := 0, 0, 0
	for {
		entry, token, err := readFrame(in)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("readFrame: %w", err)
		}
		read++
		if verr := token.Verify(entry); verr != nil {
			rejected++
			fmt.Fprintf(os.Stderr, "%s recv: REJECT (%v) ns=%x... path=%s\n", tag, verr, entry.NamespaceID[:4], formatPath(entry.Path))
			continue
		}
		if _, stored := store.Insert(entry); stored {
			accepted++
			fmt.Fprintf(os.Stderr, "%s recv: ACCEPT ns=%x... path=%s ts=%d payload_len=%d\n",
				tag, entry.NamespaceID[:4], formatPath(entry.Path), entry.Timestamp, entry.PayloadLength)
		}
	}
	fmt.Fprintf(os.Stderr, "%s recv: read=%d accepted=%d rejected=%d store_len=%d\n",
		tag, read, accepted, rejected, store.Len())
	return nil
}

// ============================================================================
// Wire framing (kept deliberately ad-hoc; this is a demo, not WGPS).
// ============================================================================

func writeFrame(w io.Writer, entry datamodel.Entry, token meadowcap.AuthorisationToken) error {
	entryBytes := entry.Encode()
	capBytes := encodeCap(token.Capability)
	if err := writeLP(w, entryBytes); err != nil {
		return err
	}
	if _, err := w.Write(token.Signature); err != nil {
		return err
	}
	return writeLP(w, capBytes)
}

func readFrame(r io.Reader) (datamodel.Entry, meadowcap.AuthorisationToken, error) {
	entryBytes, err := readLP(r)
	if err != nil {
		return datamodel.Entry{}, meadowcap.AuthorisationToken{}, err
	}
	entry, _, err := datamodel.DecodeEntry(willow25.EntrySpec(), entryBytes)
	if err != nil {
		return datamodel.Entry{}, meadowcap.AuthorisationToken{}, fmt.Errorf("decode entry: %w", err)
	}
	sig := make([]byte, ed25519.SignatureSize)
	if _, err := io.ReadFull(r, sig); err != nil {
		return datamodel.Entry{}, meadowcap.AuthorisationToken{}, fmt.Errorf("read sig: %w", err)
	}
	capBytes, err := readLP(r)
	if err != nil {
		return datamodel.Entry{}, meadowcap.AuthorisationToken{}, fmt.Errorf("read cap: %w", err)
	}
	cap, err := decodeCap(capBytes)
	if err != nil {
		return datamodel.Entry{}, meadowcap.AuthorisationToken{}, fmt.Errorf("decode cap: %w", err)
	}
	return entry, meadowcap.AuthorisationToken{Capability: cap, Signature: sig}, nil
}

func writeLP(w io.Writer, b []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func readLP(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > 1<<20 {
		return nil, fmt.Errorf("framed payload too large: %d", n)
	}
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

// encodeCap is a tiny ad-hoc serialisation of CommunalCapability: just
// mode + namespace_key + user_key + delegations. Delegations are skipped
// (none in this demo). Real wire format is encode_mc_capability — see
// TECH_DEBT.md.
func encodeCap(c meadowcap.CommunalCapability) []byte {
	var buf bytes.Buffer
	buf.WriteByte(byte(c.Mode))
	buf.Write(c.NamespaceKey)
	buf.Write(c.UserKey)
	binary.Write(&buf, binary.BigEndian, uint32(len(c.Delegations)))
	for _, d := range c.Delegations {
		areaBytes := encodeAreaSimple(d.Area)
		binary.Write(&buf, binary.BigEndian, uint32(len(areaBytes)))
		buf.Write(areaBytes)
		buf.Write(d.NewUserKey)
		buf.Write(d.Signature)
	}
	return buf.Bytes()
}

func decodeCap(b []byte) (meadowcap.CommunalCapability, error) {
	if len(b) < 1+32+32+4 {
		return meadowcap.CommunalCapability{}, fmt.Errorf("cap too short")
	}
	mode := meadowcap.AccessMode(b[0])
	cap := meadowcap.CommunalCapability{
		Mode:         mode,
		NamespaceKey: ed25519.PublicKey(b[1:33]),
		UserKey:      ed25519.PublicKey(b[33:65]),
	}
	delegCount := binary.BigEndian.Uint32(b[65:69])
	off := 69
	for i := uint32(0); i < delegCount; i++ {
		if off+4 > len(b) {
			return cap, fmt.Errorf("trunc deleg %d", i)
		}
		areaLen := binary.BigEndian.Uint32(b[off : off+4])
		off += 4
		if off+int(areaLen)+32+64 > len(b) {
			return cap, fmt.Errorf("trunc deleg %d body", i)
		}
		area, err := decodeAreaSimple(b[off : off+int(areaLen)])
		if err != nil {
			return cap, fmt.Errorf("decode area: %w", err)
		}
		off += int(areaLen)
		newReceiver := make([]byte, 32)
		copy(newReceiver, b[off:off+32])
		off += 32
		sig := make([]byte, 64)
		copy(sig, b[off:off+64])
		off += 64
		cap.Delegations = append(cap.Delegations, meadowcap.Delegation{
			Area:       area,
			NewUserKey: newReceiver,
			Signature:  sig,
		})
	}
	return cap, nil
}

// encodeAreaSimple / decodeAreaSimple are tiny self-describing area encodings
// used only by the demo's capability framing. NOT the spec's
// encode_area_in_area (that one is relative; this demo's is absolute).
func encodeAreaSimple(a datamodel.Area) []byte {
	var buf bytes.Buffer
	if a.Subspace == nil {
		buf.WriteByte(0)
	} else {
		buf.WriteByte(1)
		binary.Write(&buf, binary.BigEndian, uint32(len(*a.Subspace)))
		buf.Write(*a.Subspace)
	}
	pathBytes := a.PathPrefix.Encode()
	binary.Write(&buf, binary.BigEndian, uint32(len(pathBytes)))
	buf.Write(pathBytes)
	if a.Times.Open {
		buf.WriteByte(1)
		binary.Write(&buf, binary.BigEndian, a.Times.Start)
	} else {
		buf.WriteByte(0)
		binary.Write(&buf, binary.BigEndian, a.Times.Start)
		binary.Write(&buf, binary.BigEndian, a.Times.End)
	}
	return buf.Bytes()
}

func decodeAreaSimple(b []byte) (datamodel.Area, error) {
	off := 0
	if len(b) < 1 {
		return datamodel.Area{}, errors.New("area: truncated")
	}
	var subspace *[]byte
	if b[off] == 1 {
		off++
		if off+4 > len(b) {
			return datamodel.Area{}, errors.New("subspace len truncated")
		}
		subLen := binary.BigEndian.Uint32(b[off : off+4])
		off += 4
		if off+int(subLen) > len(b) {
			return datamodel.Area{}, errors.New("subspace bytes truncated")
		}
		s := make([]byte, subLen)
		copy(s, b[off:off+int(subLen)])
		subspace = &s
		off += int(subLen)
	} else {
		off++
	}
	if off+4 > len(b) {
		return datamodel.Area{}, errors.New("path len truncated")
	}
	pathLen := binary.BigEndian.Uint32(b[off : off+4])
	off += 4
	if off+int(pathLen) > len(b) {
		return datamodel.Area{}, errors.New("path bytes truncated")
	}
	path, _, err := datamodel.Decode(willow25.Limits(), b[off:off+int(pathLen)])
	if err != nil {
		return datamodel.Area{}, fmt.Errorf("decode path: %w", err)
	}
	off += int(pathLen)
	if off+1+8 > len(b) {
		return datamodel.Area{}, errors.New("times truncated")
	}
	openFlag := b[off]
	off++
	start := binary.BigEndian.Uint64(b[off : off+8])
	off += 8
	var times datamodel.TimeRange
	if openFlag == 1 {
		times = datamodel.NewTimeRangeOpen(start)
	} else {
		if off+8 > len(b) {
			return datamodel.Area{}, errors.New("times end truncated")
		}
		end := binary.BigEndian.Uint64(b[off : off+8])
		off += 8
		times, err = datamodel.NewTimeRangeClosed(start, end)
		if err != nil {
			return datamodel.Area{}, err
		}
	}
	return datamodel.Area{Subspace: subspace, PathPrefix: path, Times: times}, nil
}

func formatPath(p datamodel.Path) string {
	var buf bytes.Buffer
	for i := 0; i < p.ComponentCount(); i++ {
		if i > 0 {
			buf.WriteByte('/')
		}
		buf.Write(p.Component(i))
	}
	return buf.String()
}
