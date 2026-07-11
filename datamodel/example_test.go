package datamodel_test

import (
	"encoding/hex"
	"fmt"

	"github.com/Deln0r/willow-go/datamodel"
	"github.com/Deln0r/willow-go/willow25"
)

func ExamplePath_Encode() {
	p, err := willow25.NewPath([][]byte{[]byte("notes"), []byte("greeting.txt")})
	if err != nil {
		panic(err)
	}
	fmt.Println(hex.EncodeToString(p.Encode()))
	// Output:
	// c211056e6f7465736772656574696e672e747874
}

func ExampleDecode() {
	src, _ := hex.DecodeString("c211056e6f7465736772656574696e672e747874")
	p, n, err := datamodel.Decode(willow25.Limits(), src)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%d components, %d bytes consumed\n", p.ComponentCount(), n)
	for _, c := range p.Components() {
		fmt.Printf("%s\n", c)
	}
	// Output:
	// 2 components, 20 bytes consumed
	// notes
	// greeting.txt
}

func ExampleArea_EncodeRelativeTo() {
	limits := willow25.Limits()

	// The outer area: everything. The inner area: one subspace, under the
	// "chat" path prefix, any time. Encoding the inner area relative to the
	// outer one is how Willow compresses areas that share context.
	outer := datamodel.FullArea(limits)
	prefix, _ := willow25.NewPath([][]byte{[]byte("chat")})
	subspace := make([]byte, 32)
	inner := datamodel.Area{Subspace: &subspace, PathPrefix: prefix, Times: datamodel.FullTimeRange()}

	fmt.Println(hex.EncodeToString(inner.EncodeRelativeTo(outer)))
	// Output:
	// e00000000000000000000000000000000000000000000000000000000000000000004163686174
}
