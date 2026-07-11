package meadowcap_test

import (
	"bytes"
	"crypto/ed25519"
	"fmt"

	"github.com/Deln0r/willow-go/datamodel"
	"github.com/Deln0r/willow-go/meadowcap"
	"github.com/Deln0r/willow-go/willow25"
)

func ExampleCommunalCapability_AppendDelegation() {
	// Deterministic keys from fixed seeds for a reproducible example. Real
	// code must use crypto/rand.
	nsPub := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	userPriv := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	userPub := userPriv.Public().(ed25519.PublicKey)
	delegatePub := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize)).Public().(ed25519.PublicKey)

	// A communal write capability grants the genesis user their own subspace.
	writeCap, err := meadowcap.NewCommunal(meadowcap.AccessModeWrite, nsPub, userPub)
	if err != nil {
		panic(err)
	}

	// Delegate write access to the "chat" subtree of that subspace.
	subspace := append([]byte(nil), userPub...)
	prefix, _ := willow25.NewPath([][]byte{[]byte("chat")})
	area := datamodel.Area{Subspace: &subspace, PathPrefix: prefix, Times: datamodel.FullTimeRange()}

	if err := writeCap.AppendDelegation(userPriv, area, delegatePub); err != nil {
		panic(err)
	}

	fmt.Println("valid:", writeCap.IsValid())
	fmt.Println("delegations:", len(writeCap.Delegations))
	// Output:
	// valid: true
	// delegations: 1
}
