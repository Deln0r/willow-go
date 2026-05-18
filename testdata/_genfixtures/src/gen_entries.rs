//! Generate Willow entry fixtures from upstream `willow_data_model`. Uses a
//! local `Bytes32` newtype for the namespace/subspace/digest slots so the
//! harness does not depend on willow25.

mod bytes32;

use std::fs;
use std::path::PathBuf;

use serde::Serialize;
use ufotofu::IntoConsumer;
use ufotofu::codec::Encodable;
use willow_data_model::prelude::{Entry, Entrylike, Keylike, Namespaced, Path};
use willow_data_model::groupings::Coordinatelike;

use bytes32::Bytes32;

const MCL: usize = 4096;
const MCC: usize = 4096;
const MPL: usize = 4096;
const ID_WIDTH: usize = 32;

#[derive(Serialize)]
struct EntryParams {
    mcl: usize,
    mcc: usize,
    mpl: usize,
    namespace_id_width: usize,
    subspace_id_width: usize,
    payload_digest_width: usize,
}

#[derive(Serialize)]
struct EntryCase {
    name: String,
    namespace_id_hex: String,
    subspace_id_hex: String,
    path_components_hex: Vec<String>,
    timestamp: u64,
    payload_length: u64,
    payload_digest_hex: String,
    encoded_hex: String,
}

#[derive(Serialize)]
struct EntryFile {
    params: EntryParams,
    cases: Vec<EntryCase>,
}

fn build_entry(
    namespace: [u8; 32],
    subspace: [u8; 32],
    path_components: &[&[u8]],
    timestamp: u64,
    payload_length: u64,
    payload_digest: [u8; 32],
) -> Entry<MCL, MCC, MPL, Bytes32, Bytes32, Bytes32> {
    let path = Path::<MCL, MCC, MPL>::from_slices(path_components).expect("valid path");
    Entry::builder()
        .namespace_id(Bytes32(namespace))
        .subspace_id(Bytes32(subspace))
        .path(path)
        .timestamp(timestamp)
        .payload_length(payload_length)
        .payload_digest(Bytes32(payload_digest))
        .build()
}

fn encode_entry(entry: &Entry<MCL, MCC, MPL, Bytes32, Bytes32, Bytes32>) -> Vec<u8> {
    let mut buf: Vec<u8> = Vec::new();
    {
        let mut consumer = (&mut buf).into_consumer();
        pollster::block_on(entry.encode(&mut consumer)).expect("IntoVec encode infallible");
    }
    buf
}

fn write_json<T: Serialize>(path: PathBuf, value: &T) {
    let json = serde_json::to_string_pretty(value).expect("serialize");
    fs::write(&path, format!("{json}\n")).expect("write fixture");
    println!("wrote {}", path.display());
}

fn out_dir() -> PathBuf {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir.parent().expect("testdata/").join("entries")
}

fn case_from_entry(
    name: &str,
    entry: Entry<MCL, MCC, MPL, Bytes32, Bytes32, Bytes32>,
) -> EntryCase {
    let encoded = encode_entry(&entry);
    EntryCase {
        name: name.to_string(),
        namespace_id_hex: hex::encode(entry.wdm_namespace_id().0),
        subspace_id_hex: hex::encode(entry.wdm_subspace_id().0),
        path_components_hex: entry
            .wdm_path()
            .components()
            .map(|c| hex::encode(c.as_ref()))
            .collect(),
        timestamp: entry.wdm_timestamp().into(),
        payload_length: entry.wdm_payload_length(),
        payload_digest_hex: hex::encode(entry.wdm_payload_digest().0),
        encoded_hex: hex::encode(encoded),
    }
}

fn gen_basic() {
    let zeros = [0u8; 32];
    let ones = [0x11u8; 32];
    let pattern_a: [u8; 32] = {
        let mut a = [0u8; 32];
        for (i, b) in a.iter_mut().enumerate() {
            *b = i as u8;
        }
        a
    };
    let pattern_b: [u8; 32] = [0xABu8; 32];

    let cases = vec![
        case_from_entry(
            "all_zeros_empty_path_t0_p0",
            build_entry(zeros, zeros, &[], 0, 0, zeros),
        ),
        case_from_entry(
            "all_ones_single_component_small_t_small_pl",
            build_entry(ones, ones, &[b"alfie"], 12345, 17, ones),
        ),
        case_from_entry(
            "pattern_ids_multi_component_path",
            build_entry(
                pattern_a,
                pattern_b,
                &[b"vacation", b"plan"],
                1_700_000_000_000_000,
                4096,
                pattern_a,
            ),
        ),
        case_from_entry(
            "timestamp_boundary_251_standalone_inline",
            build_entry(zeros, ones, &[b"t"], 251, 1, zeros),
        ),
        case_from_entry(
            "timestamp_boundary_252_standalone_1byte_follow",
            build_entry(zeros, ones, &[b"t"], 252, 1, zeros),
        ),
        case_from_entry(
            "timestamp_boundary_65535_standalone_2byte_follow",
            build_entry(zeros, ones, &[b"t"], 65535, 1, zeros),
        ),
        case_from_entry(
            "timestamp_boundary_65536_standalone_4byte_follow",
            build_entry(zeros, ones, &[b"t"], 65536, 1, zeros),
        ),
        case_from_entry(
            "payload_length_boundary_4294967295_standalone_4byte_follow",
            build_entry(zeros, zeros, &[], 1, (1u64 << 32) - 1, ones),
        ),
        case_from_entry(
            "payload_length_boundary_4294967296_standalone_8byte_follow",
            build_entry(zeros, zeros, &[], 1, 1u64 << 32, ones),
        ),
        case_from_entry(
            "path_with_zero_bytes_in_component",
            build_entry(
                ones,
                pattern_b,
                &[b"folder", b"\x00\x00\x00", b"file.txt"],
                42,
                100,
                pattern_a,
            ),
        ),
    ];

    let file = EntryFile {
        params: EntryParams {
            mcl: MCL,
            mcc: MCC,
            mpl: MPL,
            namespace_id_width: ID_WIDTH,
            subspace_id_width: ID_WIDTH,
            payload_digest_width: ID_WIDTH,
        },
        cases,
    };

    write_json(out_dir().join("basic.json"), &file);
}

fn main() {
    fs::create_dir_all(out_dir()).expect("ensure entries/");
    gen_basic();
    println!("done.");
}
