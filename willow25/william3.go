package willow25

import (
	"encoding/binary"
	"math/bits"
)

// William3 is the BLAKE3-with-custom-IV hash that willow25 names as its
// payload digest (see https://worm-blossom.github.io/bab/#instantiations_william).
// It uses the BLAKE3 compression function unchanged, but substitutes a
// distinct IV — making it domain-separated from vanilla BLAKE3 — and
// arranges chunks into the same binary tree BLAKE3 uses, with the same
// 1024-byte chunk size.
//
// This file ports bab_rs 0.8.0's batch_hash. bab_rs 0.5.0 (which an earlier
// version of this file tracked) computed WILLIAM3 incorrectly: it did not
// compress a block for empty input and passed a fixed block length of 64 to
// the compression instead of the real (possibly partial) block length. 0.8.0
// fixes both. Verified byte-identical against the upstream william3vectors.txt.

const (
	william3BlockLen  = 64
	william3ChunkSize = 1024

	flagChunkStart uint32 = 1 << 0
	flagChunkEnd   uint32 = 1 << 1
	flagParent     uint32 = 1 << 2
	flagRoot       uint32 = 1 << 3
)

// william3IV is the WILLIAM3-specific initial chaining value, distinct from
// the BLAKE3 IV. Per bab_rs/src/william3/basics.rs.
var william3IV = [8]uint32{
	0xc88f633b, 0x4168fbf2, 0x6ba32583, 0xb0ff1847,
	0xac57e47d, 0xa8931330, 0x796a4645, 0x6b28a3ee,
}

// blake3MsgSchedule is the standard BLAKE3 message permutation schedule,
// reused unchanged by William3.
var blake3MsgSchedule = [7][16]int{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	{2, 6, 3, 10, 7, 0, 4, 13, 1, 11, 12, 5, 9, 14, 15, 8},
	{3, 4, 10, 12, 13, 2, 7, 14, 6, 5, 9, 0, 11, 15, 8, 1},
	{10, 7, 12, 9, 14, 3, 13, 15, 4, 0, 11, 2, 5, 8, 1, 6},
	{12, 13, 9, 11, 15, 10, 14, 8, 7, 2, 5, 3, 0, 1, 6, 4},
	{9, 14, 11, 5, 8, 12, 15, 1, 13, 3, 0, 10, 2, 6, 4, 7},
	{11, 15, 5, 0, 1, 9, 8, 6, 14, 10, 2, 12, 3, 4, 7, 13},
}

// William3Sum returns the 32-byte WILLIAM3 digest of input.
func William3Sum(input []byte) [32]byte {
	var out [32]byte
	doBatchHash(input, &out, true)
	return out
}

func doBatchHash(input []byte, out *[32]byte, isRoot bool) {
	if len(input) <= william3ChunkSize {
		hashChunk(input, isRoot, out)
		return
	}
	leftLen := splitPoint(len(input))
	var left, right [32]byte
	doBatchHash(input[:leftLen], &left, false)
	doBatchHash(input[leftLen:], &right, false)
	hashInner(left, right, uint64(len(input)), isRoot, out)
}

// splitPoint returns the greatest power of two strictly less than n, except
// when n itself is a power of two — then n/2.
func splitPoint(n int) int {
	if n&(n-1) == 0 {
		return n / 2
	}
	// Greatest power of two strictly less than n = 1 << floor(log2(n)).
	return 1 << (bits.Len(uint(n)) - 1)
}

func hashChunk(chunk []byte, isRoot bool, out *[32]byte) {
	flagsStart := flagChunkStart
	flagsEnd := flagChunkEnd
	if isRoot {
		flagsEnd |= flagRoot
	}
	hash1(chunk, william3IV, 0, 0, flagsStart, flagsEnd, out)
}

func hashInner(left, right [32]byte, subtreeLen uint64, isRoot bool, out *[32]byte) {
	flags := flagParent
	if isRoot {
		flags |= flagRoot
	}
	var msg [64]byte
	copy(msg[:32], left[:])
	copy(msg[32:], right[:])
	hash1(msg[:], william3IV, subtreeLen, flags, 0, 0, out)
}

// hash1 processes input as a sequence of 64-byte blocks, compressing each
// into the chaining value cv (which starts as `key`). The very first block
// gets `flagsStart` OR'd into its flags; the very last block gets
// `flagsEnd` OR'd in. The final block is zero-padded if shorter than 64
// bytes. After all blocks, the chaining value is serialised little-endian
// into out.
func hash1(input []byte, key [8]uint32, counter uint64, flags, flagsStart, flagsEnd uint32, out *[32]byte) {
	cv := key
	blockFlags := flags | flagsStart

	if len(input) == 0 {
		// Empty input still compresses one zero-padded block of length 0,
		// carrying both the start and end flags.
		blockFlags |= flagsEnd
		var only [william3BlockLen]byte
		compressInPlace(&cv, &only, 0, counter, blockFlags)
	} else {
		slice := input
		for len(slice) > 0 {
			var block [william3BlockLen]byte
			take := len(slice)
			if take > william3BlockLen {
				take = william3BlockLen
			}
			if len(slice) <= william3BlockLen {
				blockFlags |= flagsEnd
			}
			copy(block[:], slice[:take])
			// The block length passed to the compression is the actual byte
			// count of this (possibly partial, zero-padded) block.
			compressInPlace(&cv, &block, uint32(take), counter, blockFlags)
			blockFlags = flags
			slice = slice[take:]
		}
	}

	for i, w := range cv {
		binary.LittleEndian.PutUint32(out[i*4:(i+1)*4], w)
	}
}

func compressInPlace(cv *[8]uint32, block *[64]byte, blockLen uint32, counter uint64, flags uint32) {
	var msg [16]uint32
	for i := 0; i < 16; i++ {
		msg[i] = binary.LittleEndian.Uint32(block[i*4 : (i+1)*4])
	}
	state := [16]uint32{
		cv[0], cv[1], cv[2], cv[3],
		cv[4], cv[5], cv[6], cv[7],
		william3IV[0], william3IV[1], william3IV[2], william3IV[3],
		uint32(counter), uint32(counter >> 32), blockLen, flags,
	}
	for r := 0; r < 7; r++ {
		round(&state, &msg, r)
	}
	cv[0] = state[0] ^ state[8]
	cv[1] = state[1] ^ state[9]
	cv[2] = state[2] ^ state[10]
	cv[3] = state[3] ^ state[11]
	cv[4] = state[4] ^ state[12]
	cv[5] = state[5] ^ state[13]
	cv[6] = state[6] ^ state[14]
	cv[7] = state[7] ^ state[15]
}

func round(state *[16]uint32, msg *[16]uint32, r int) {
	s := blake3MsgSchedule[r]
	g(state, 0, 4, 8, 12, msg[s[0]], msg[s[1]])
	g(state, 1, 5, 9, 13, msg[s[2]], msg[s[3]])
	g(state, 2, 6, 10, 14, msg[s[4]], msg[s[5]])
	g(state, 3, 7, 11, 15, msg[s[6]], msg[s[7]])
	g(state, 0, 5, 10, 15, msg[s[8]], msg[s[9]])
	g(state, 1, 6, 11, 12, msg[s[10]], msg[s[11]])
	g(state, 2, 7, 8, 13, msg[s[12]], msg[s[13]])
	g(state, 3, 4, 9, 14, msg[s[14]], msg[s[15]])
}

func g(state *[16]uint32, a, b, c, d int, x, y uint32) {
	state[a] = state[a] + state[b] + x
	state[d] = bits.RotateLeft32(state[d]^state[a], -16)
	state[c] = state[c] + state[d]
	state[b] = bits.RotateLeft32(state[b]^state[c], -12)
	state[a] = state[a] + state[b] + y
	state[d] = bits.RotateLeft32(state[d]^state[a], -8)
	state[c] = state[c] + state[d]
	state[b] = bits.RotateLeft32(state[b]^state[c], -7)
}
