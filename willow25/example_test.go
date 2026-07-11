package willow25_test

import (
	"encoding/hex"
	"fmt"

	"github.com/Deln0r/willow-go/willow25"
)

func ExampleHashPayload() {
	digest := willow25.HashPayload([]byte("hello, willow"))
	fmt.Println(hex.EncodeToString(digest[:]))
	// Output:
	// 79ae22083788320af5801eaab3becf3b8c044d0d5523f93e1f3150aee4481017
}
