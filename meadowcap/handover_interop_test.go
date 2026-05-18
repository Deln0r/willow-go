package meadowcap

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Deln0r/willow-go/datamodel"
	"github.com/Deln0r/willow-go/willow25"
)

type handoverAreaJSON struct {
	SubspaceHex       *string  `json:"subspace_hex"`
	PathComponentsHex []string `json:"path_components_hex"`
	TimesStart        uint64   `json:"times_start"`
	TimesEnd          *uint64  `json:"times_end"`
}

type handoverDelegationStep struct {
	NewArea         handoverAreaJSON `json:"new_area"`
	NewReceiverHex  string           `json:"new_receiver_hex"`
	SignatureHex    string           `json:"signature_hex"`
}

type handoverChainFixture struct {
	Name              string                   `json:"name"`
	AccessMode        uint8                    `json:"access_mode"`
	NamespaceKeyHex   string                   `json:"namespace_key_hex"`
	GenesisUserKeyHex string                   `json:"genesis_user_key_hex"`
	ReceiverSeedsHex  []string                 `json:"receiver_seeds_hex"`
	Delegations       []handoverDelegationStep `json:"delegations"`
}

type handoverFile struct {
	Cases []handoverChainFixture `json:"cases"`
}

func areaFromHandoverJSON(t *testing.T, j handoverAreaJSON) datamodel.Area {
	t.Helper()
	limits := willow25.Limits()
	var sub *[]byte
	if j.SubspaceHex != nil {
		b, err := hex.DecodeString(*j.SubspaceHex)
		if err != nil {
			t.Fatalf("subspace hex: %v", err)
		}
		sub = &b
	}
	pathComps := make([][]byte, len(j.PathComponentsHex))
	for i, c := range j.PathComponentsHex {
		b, err := hex.DecodeString(c)
		if err != nil {
			t.Fatalf("path component hex: %v", err)
		}
		pathComps[i] = b
	}
	path, err := datamodel.FromSlices(limits, pathComps)
	if err != nil {
		t.Fatalf("FromSlices: %v", err)
	}
	var times datamodel.TimeRange
	if j.TimesEnd == nil {
		times = datamodel.NewTimeRangeOpen(j.TimesStart)
	} else {
		times, err = datamodel.NewTimeRangeClosed(j.TimesStart, *j.TimesEnd)
		if err != nil {
			t.Fatalf("times: %v", err)
		}
	}
	return datamodel.Area{Subspace: sub, PathPrefix: path, Times: times}
}

// TestDelegation_InteropFixtures is the cross-impl byte-compat probe for
// chunk 10. The Rust harness builds delegation chains using
// `meadowcap::WriteCapability::delegate`, which signs over the spec-defined
// handover bytes computed by `create_handover`. We reconstruct each chain
// here and call IsValid; success means our handoverBytesAt produces
// byte-identical output to upstream's create_handover (signatures are over
// these bytes; if our computation diverged, the signatures would fail to
// verify here).
func TestDelegation_InteropFixtures(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "meadowcap", "delegation_chains.json"))
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var f handoverFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, c := range f.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()

			nsKey, err := hex.DecodeString(c.NamespaceKeyHex)
			if err != nil {
				t.Fatalf("namespace key hex: %v", err)
			}
			userKey, err := hex.DecodeString(c.GenesisUserKeyHex)
			if err != nil {
				t.Fatalf("user key hex: %v", err)
			}

			mode := AccessModeWrite
			if c.AccessMode == 0 {
				mode = AccessModeRead
			}
			cap, err := NewCommunal(mode, nsKey, userKey)
			if err != nil {
				t.Fatalf("NewCommunal: %v", err)
			}

			for i, step := range c.Delegations {
				newArea := areaFromHandoverJSON(t, step.NewArea)
				newReceiver, err := hex.DecodeString(step.NewReceiverHex)
				if err != nil {
					t.Fatalf("step %d new_receiver hex: %v", i, err)
				}
				sig, err := hex.DecodeString(step.SignatureHex)
				if err != nil {
					t.Fatalf("step %d signature hex: %v", i, err)
				}
				cap.Delegations = append(cap.Delegations, Delegation{
					Area:       newArea,
					NewUserKey: ed25519.PublicKey(newReceiver),
					Signature:  sig,
				})
			}

			if !cap.IsValid() {
				t.Errorf("Rust-built chain failed IsValid in Go — handover bytes diverge")
			}
		})
	}
}

// TestDelegation_InteropFixtures_SeedsRoundTrip is a sanity check that the
// receiver seeds we emit in the fixture actually correspond to the public
// keys recorded as new_receiver. If this fails, the harness is broken (not
// our Go side).
func TestDelegation_InteropFixtures_SeedsRoundTrip(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "meadowcap", "delegation_chains.json"))
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var f handoverFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range f.Cases {
		if len(c.ReceiverSeedsHex) != len(c.Delegations) {
			t.Errorf("%s: %d seeds vs %d delegations", c.Name, len(c.ReceiverSeedsHex), len(c.Delegations))
			continue
		}
		for i, seedHex := range c.ReceiverSeedsHex {
			seed, _ := hex.DecodeString(seedHex)
			privKey := ed25519.NewKeyFromSeed(seed)
			wantPub, _ := hex.DecodeString(c.Delegations[i].NewReceiverHex)
			gotPub := privKey.Public().(ed25519.PublicKey)
			if string(gotPub) != string(wantPub) {
				t.Errorf("%s delegation %d: seed-derived pubkey != recorded receiver", c.Name, i)
			}
		}
	}
}
