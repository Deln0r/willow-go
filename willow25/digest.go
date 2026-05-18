package willow25

// HashPayload returns the 32-byte WILLIAM3 digest of payload — the
// payload-digest function defined by willow25
// (https://willowprotocol.org/specs/willow25/index.html#willow25_data_model).
// WILLIAM3 is the BLAKE3 compression function with a substituted IV, so
// payloads produce DIFFERENT digests under WILLIAM3 vs vanilla BLAKE3.
// Implementation lives in william3.go; verified byte-identical against
// upstream bab_rs v0.5.0 on the testdata/william3/ fixtures.
func HashPayload(payload []byte) [PayloadDigestWidth]byte {
	return William3Sum(payload)
}
