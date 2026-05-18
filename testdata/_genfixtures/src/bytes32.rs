//! A minimal `[u8; 32]` newtype with the ufotofu Encodable traits the
//! upstream `Entry<MCL, MCC, MPL, N, S, PD>` requires for *encoding*. We do
//! not implement Decodable — the harness only generates fixtures, never
//! parses them.
//!
//! The 32-byte width matches the willow25 NamespaceId / SubspaceId /
//! PayloadDigest specialisations, but we avoid pulling in the willow25 crate
//! (with its ed25519/blake3/fjall deps) into the harness.

use ufotofu::codec_prelude::*;

#[derive(Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Debug)]
pub struct Bytes32(pub [u8; 32]);

impl Encodable for Bytes32 {
    async fn encode<C>(&self, consumer: &mut C) -> Result<(), C::Error>
    where
        C: BulkConsumer<Item = u8> + ?Sized,
    {
        consumer
            .bulk_consume_full_slice(&self.0)
            .await
            .map_err(|err| err.into_reason())
    }
}

impl EncodableKnownLength for Bytes32 {
    fn len_of_encoding(&self) -> usize {
        32
    }
}
