package willow25

import "lukechampine.com/blake3"

// HashPayload returns the 32-byte BLAKE3 digest of payload. This stands in
// for the Willow'25 William3 digest (a BLAKE3 variant with custom domain
// separation) until that variant is implemented; for end-to-end
// compatibility with willow_rs payloads, see TECH_DEBT.md.
func HashPayload(payload []byte) [PayloadDigestWidth]byte {
	return blake3.Sum256(payload)
}
