//! Generate Meadowcap delegation chain fixtures using the upstream
//! `meadowcap::WriteCapability::delegate` API with real Ed25519 keys. This
//! is the cross-impl interop probe for chunk 10: the resulting chain has
//! signatures computed by willow_rs's `create_handover` function over the
//! spec-defined handover bytes; the Go test verifies them with our own
//! handover-byte computation. If our bytes match willow_rs's, the
//! signatures verify; if they diverge, verification fails.
//!
//! Each fixture spec includes deterministic Ed25519 keypair seeds (random
//! per run is fine since both sides use the same seeds in this file) plus
//! the chain shape and the resulting signatures.

mod bytes32;

use std::fs;
use std::path::PathBuf;

use ed25519_dalek::{Signature, Signer, SigningKey, Verifier, VerifyingKey, SECRET_KEY_LENGTH};
use meadowcap::raw::AccessMode;
use meadowcap::WriteCapability;
use rand::rngs::OsRng;
use rand::RngCore;
use serde::Serialize;
use ufotofu::codec_prelude::*;
use willow_data_model::groupings::{Area, WillowRange};
use willow_data_model::prelude::Path;

const MCL: usize = 4096;
const MCC: usize = 4096;
const MPL: usize = 4096;

// ============================================================================
// Wrapper types implementing the trait bounds meadowcap requires for keys
// and signatures. Each is a thin newtype around an ed25519-dalek type with
// the willow_data_model Encodable / EncodableKnownLength impls bolted on,
// plus signature::Signer / Verifier / Keypair where needed.
// ============================================================================

#[derive(Clone, PartialEq, Eq, Hash, Debug)]
struct PubKey(VerifyingKey);

impl Encodable for PubKey {
    async fn encode<C>(&self, consumer: &mut C) -> Result<(), C::Error>
    where
        C: BulkConsumer<Item = u8> + ?Sized,
    {
        consumer
            .bulk_consume_full_slice(self.0.as_bytes())
            .await
            .map_err(|err| err.into_reason())
    }
}

impl EncodableKnownLength for PubKey {
    fn len_of_encoding(&self) -> usize {
        32
    }
}

impl Verifier<Sig> for PubKey {
    fn verify(&self, msg: &[u8], signature: &Sig) -> Result<(), signature::Error> {
        Verifier::<Signature>::verify(&self.0, msg, &signature.0)
    }
}

#[derive(Clone, PartialEq, Eq, Debug)]
struct Sig(Signature);

impl Encodable for Sig {
    async fn encode<C>(&self, consumer: &mut C) -> Result<(), C::Error>
    where
        C: BulkConsumer<Item = u8> + ?Sized,
    {
        consumer
            .bulk_consume_full_slice(&self.0.to_bytes())
            .await
            .map_err(|err| err.into_reason())
    }
}

impl EncodableKnownLength for Sig {
    fn len_of_encoding(&self) -> usize {
        64
    }
}

struct Keypair {
    signing: SigningKey,
}

impl Keypair {
    fn new(seed: [u8; SECRET_KEY_LENGTH]) -> Self {
        Self {
            signing: SigningKey::from_bytes(&seed),
        }
    }
    fn public(&self) -> PubKey {
        PubKey(self.signing.verifying_key())
    }
}

impl signature::Keypair for Keypair {
    type VerifyingKey = PubKey;
    fn verifying_key(&self) -> PubKey {
        PubKey(self.signing.verifying_key())
    }
}

impl Signer<Sig> for Keypair {
    fn try_sign(&self, msg: &[u8]) -> Result<Sig, signature::Error> {
        Ok(Sig(self.signing.sign(msg)))
    }
}

// ============================================================================
// Fixture JSON shape.
// ============================================================================

#[derive(Serialize)]
struct DelegationStep {
    new_area: AreaJson,
    new_receiver_hex: String,
    signature_hex: String,
}

#[derive(Serialize, Clone)]
struct AreaJson {
    subspace_hex: Option<String>,
    path_components_hex: Vec<String>,
    times_start: u64,
    times_end: Option<u64>,
}

#[derive(Serialize)]
struct ChainFixture {
    name: String,
    access_mode: u8, // 0 read, 1 write
    namespace_key_hex: String,
    /// Genesis user key (root receiver) as hex.
    genesis_user_key_hex: String,
    /// Seeds for each subsequent receiver's keypair — needed so the Go test
    /// has the private key for the cap's effective receiver if it wants to
    /// also sign downstream entries.
    receiver_seeds_hex: Vec<String>,
    delegations: Vec<DelegationStep>,
}

#[derive(Serialize)]
struct File {
    cases: Vec<ChainFixture>,
}

fn make_path(slices: &[&[u8]]) -> Path<MCL, MCC, MPL> {
    Path::<MCL, MCC, MPL>::from_slices(slices).expect("valid path")
}

fn full_subspace_area(subspace: &PubKey) -> Area<MCL, MCC, MPL, PubKey> {
    Area::new(Some(subspace.clone()), make_path(&[]), WillowRange::full())
}

fn subspace_area_with_path(
    subspace: &PubKey,
    path_slices: &[&[u8]],
) -> Area<MCL, MCC, MPL, PubKey> {
    Area::new(
        Some(subspace.clone()),
        make_path(path_slices),
        WillowRange::full(),
    )
}

fn area_to_json(a: &Area<MCL, MCC, MPL, PubKey>) -> AreaJson {
    AreaJson {
        subspace_hex: a.subspace().map(|s| hex::encode(s.0.as_bytes())),
        path_components_hex: a
            .path()
            .components()
            .map(|c| hex::encode(c.as_ref()))
            .collect(),
        times_start: u64::from(*a.times().start()),
        times_end: a.times().end().map(|t| u64::from(*t)),
    }
}

fn random_seed() -> [u8; SECRET_KEY_LENGTH] {
    let mut seed = [0u8; SECRET_KEY_LENGTH];
    OsRng.fill_bytes(&mut seed);
    seed
}

fn build_chain(
    name: &str,
    mode: AccessMode,
    namespace_seed: [u8; SECRET_KEY_LENGTH],
    genesis_user_seed: [u8; SECRET_KEY_LENGTH],
    steps: Vec<(Vec<Vec<u8>>, [u8; SECRET_KEY_LENGTH])>,
) -> ChainFixture {
    let namespace_kp = Keypair::new(namespace_seed);
    let genesis_kp = Keypair::new(genesis_user_seed);
    let namespace_key = namespace_kp.public();
    let genesis_user_key = genesis_kp.public();

    let mut cap: WriteCapability<MCL, MCC, MPL, PubKey, Sig, PubKey, Sig> =
        WriteCapability::new_communal(namespace_key.clone(), genesis_user_key.clone());

    let mut delegations = Vec::new();
    let mut receiver_seeds = Vec::new();
    let mut prev_kp = genesis_kp;

    for (path_components, next_seed) in steps {
        let slices: Vec<&[u8]> = path_components.iter().map(|c| c.as_slice()).collect();
        let new_area = subspace_area_with_path(&genesis_user_key, &slices);
        let new_kp = Keypair::new(next_seed);
        let new_receiver = new_kp.public();

        cap.delegate(&prev_kp, new_area.clone(), new_receiver.clone());

        let signature = cap
            .delegations()
            .last()
            .expect("just appended")
            .signature
            .clone();

        delegations.push(DelegationStep {
            new_area: area_to_json(&new_area),
            new_receiver_hex: hex::encode(new_receiver.0.as_bytes()),
            signature_hex: hex::encode(signature.0.to_bytes()),
        });
        receiver_seeds.push(hex::encode(next_seed));
        prev_kp = new_kp;
    }

    // WriteCapability is guaranteed valid by construction in upstream
    // (delegate() validates on append; the type is "validated" wrapper
    // around PossiblyValidWriteCapability).
    let _ = full_subspace_area(&genesis_user_key);

    ChainFixture {
        name: name.to_string(),
        access_mode: match mode {
            AccessMode::Read => 0,
            AccessMode::Write => 1,
        },
        namespace_key_hex: hex::encode(namespace_key.0.as_bytes()),
        genesis_user_key_hex: hex::encode(genesis_user_key.0.as_bytes()),
        receiver_seeds_hex: receiver_seeds,
        delegations,
    }
}

fn out_dir() -> PathBuf {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .parent()
        .expect("testdata/")
        .join("meadowcap")
}

fn main() {
    let cases = vec![
        build_chain(
            "communal_write_no_delegations",
            AccessMode::Write,
            random_seed(),
            random_seed(),
            vec![],
        ),
        build_chain(
            "communal_write_single_delegation_full_area",
            AccessMode::Write,
            random_seed(),
            random_seed(),
            vec![(vec![], random_seed())],
        ),
        build_chain(
            "communal_write_single_delegation_path_narrowed",
            AccessMode::Write,
            random_seed(),
            random_seed(),
            vec![(vec![b"folder".to_vec()], random_seed())],
        ),
        build_chain(
            "communal_write_two_step_root_to_bob_to_carol",
            AccessMode::Write,
            random_seed(),
            random_seed(),
            vec![
                (vec![], random_seed()),                                       // root -> bob: full area
                (vec![b"shared".to_vec(), b"docs".to_vec()], random_seed()),   // bob -> carol: narrowed
            ],
        ),
        // Read-mode chain intentionally omitted: meadowcap's
        // WriteCapability hard-wires Write. Generating a ReadCapability
        // fixture would require a parallel build_chain pathway for
        // ReadCapability. Tracked in TECH_DEBT.md as future work.
    ];

    let file = File { cases };

    fs::create_dir_all(out_dir()).expect("ensure meadowcap/");
    let target = out_dir().join("delegation_chains.json");
    let json = serde_json::to_string_pretty(&file).expect("serialize");
    fs::write(&target, format!("{json}\n")).expect("write fixture");
    println!("wrote {}", target.display());
}
