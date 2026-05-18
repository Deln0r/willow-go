//! Generate Willow path fixtures from upstream `willow_data_model` for use
//! as golden vectors by the Go test suite. Output: testdata/paths/*.json.
//!
//! Usage: `cargo run --bin gen-paths` from the directory of this manifest.
//! Re-run whenever the upstream pinned revision changes.

use std::fs;
use std::path::PathBuf;

use serde::Serialize;
use ufotofu::IntoConsumer;
use ufotofu::codec::Encodable;
use ufotofu::codec_relative::RelativeEncodable;
use willow_data_model::prelude::{Path, PathError};

#[derive(Serialize)]
struct Params {
    mcl: usize,
    mcc: usize,
    mpl: usize,
}

#[derive(Serialize)]
struct AbsoluteCase {
    name: String,
    components_hex: Vec<String>,
    encoded_hex: String,
}

#[derive(Serialize)]
struct RelativeCase {
    name: String,
    reference_components_hex: Vec<String>,
    target_components_hex: Vec<String>,
    encoded_hex: String,
}

#[derive(Serialize)]
struct AbsoluteFile {
    params: Params,
    cases: Vec<AbsoluteCase>,
}

#[derive(Serialize)]
struct RelativeFile {
    params: Params,
    cases: Vec<RelativeCase>,
}

fn encode_absolute<const MCL: usize, const MCC: usize, const MPL: usize>(
    path: &Path<MCL, MCC, MPL>,
) -> Vec<u8> {
    let mut buf: Vec<u8> = Vec::new();
    {
        let mut consumer = (&mut buf).into_consumer();
        pollster::block_on(path.encode(&mut consumer)).expect("IntoVec encode infallible");
    }
    buf
}

fn encode_relative<const MCL: usize, const MCC: usize, const MPL: usize>(
    target: &Path<MCL, MCC, MPL>,
    reference: &Path<MCL, MCC, MPL>,
) -> Vec<u8> {
    let mut buf: Vec<u8> = Vec::new();
    {
        let mut consumer = (&mut buf).into_consumer();
        pollster::block_on(target.relative_encode(reference, &mut consumer))
            .expect("IntoVec encode infallible");
    }
    buf
}

fn components_to_hex<const MCL: usize, const MCC: usize, const MPL: usize>(
    path: &Path<MCL, MCC, MPL>,
) -> Vec<String> {
    path.components().map(|c| hex::encode(c.as_ref())).collect()
}

fn make_path<const MCL: usize, const MCC: usize, const MPL: usize>(
    slices: &[&[u8]],
) -> Result<Path<MCL, MCC, MPL>, PathError> {
    Path::<MCL, MCC, MPL>::from_slices(slices)
}

fn write_json<T: Serialize>(path: PathBuf, value: &T) {
    let json = serde_json::to_string_pretty(value).expect("serialize");
    fs::write(&path, format!("{json}\n")).expect("write fixture");
    println!("wrote {}", path.display());
}

fn out_dir() -> PathBuf {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .parent()
        .expect("testdata/")
        .join("paths")
}

fn gen_basic() {
    const MCL: usize = 4096;
    const MCC: usize = 4096;
    const MPL: usize = 4096;

    let make = |name: &str, slices: &[&[u8]]| -> AbsoluteCase {
        let path = make_path::<MCL, MCC, MPL>(slices).expect("valid path");
        AbsoluteCase {
            name: name.to_string(),
            components_hex: components_to_hex(&path),
            encoded_hex: hex::encode(encode_absolute(&path)),
        }
    };

    // Varint boundary scaffolding: the 4-bit CompactU64 header tag is inline
    // for values 0..=11 and uses 1/2/4/8 follow-up bytes for tag 12/13/14/15.
    // For path_length we craft single-component paths whose byte length lands
    // exactly at each boundary; component_count uses 4096 max components per
    // willow25.
    // Pre-allocate one-byte components to satisfy the borrow checker — slices
    // referenced by `make_path` must outlive the helper call.
    let owned_bytes_12: Vec<[u8; 1]> = (0..12).map(|i| [i as u8]).collect();
    let all_12: Vec<&[u8]> = owned_bytes_12.iter().map(|b| &b[..]).collect();
    let many_one_byte_components_12: &[&[u8]] = &all_12;
    let many_one_byte_components_11: &[&[u8]] = &all_12[..11];
    let component_11_bytes: &[u8] = &[0x55; 11];
    let component_12_bytes: &[u8] = &[0x55; 12];
    let component_255_bytes: &[u8] = &[0x66; 255];
    let component_256_bytes: &[u8] = &[0x66; 256];

    let file = AbsoluteFile {
        params: Params { mcl: MCL, mcc: MCC, mpl: MPL },
        cases: vec![
            make("empty_path", &[]),
            make("single_empty_component", &[b""]),
            make("single_one_byte", &[b"a"]),
            make("multi_two_short", &[b"ab", b"cd"]),
            make("multi_three_with_zero_byte", &[b"x", b"\x00\x01\x02", b"yz"]),
            make("single_64_bytes", &[&[0xAA; 64]]),
            make("path_length_11_tag_boundary_last_inline", &[component_11_bytes]),
            make("path_length_12_tag_boundary_first_follow_byte", &[component_12_bytes]),
            make("path_length_255_tag_boundary_last_1byte_follow", &[component_255_bytes]),
            make("path_length_256_tag_boundary_first_2byte_follow", &[component_256_bytes]),
            make("component_count_11_tag_boundary_last_inline", many_one_byte_components_11),
            make("component_count_12_tag_boundary_first_follow_byte", many_one_byte_components_12),
            make("standalone_cu64_component_len_252", &[&[0x77; 252], b"tail"]),
        ],
    };

    write_json(out_dir().join("basic.json"), &file);
}

fn gen_limits() {
    // Deliberately tight params so boundary cases stay compact.
    const MCL: usize = 8;
    const MCC: usize = 4;
    const MPL: usize = 16;

    let make = |name: &str, slices: &[&[u8]]| -> AbsoluteCase {
        let path = make_path::<MCL, MCC, MPL>(slices).expect("valid path");
        AbsoluteCase {
            name: name.to_string(),
            components_hex: components_to_hex(&path),
            encoded_hex: hex::encode(encode_absolute(&path)),
        }
    };

    let file = AbsoluteFile {
        params: Params { mcl: MCL, mcc: MCC, mpl: MPL },
        cases: vec![
            make("max_component_length_exactly", &[&[0x42; 8]]),
            make("max_component_count_exactly", &[b"a", b"b", b"c", b"d"]),
            make("max_path_length_exactly", &[&[0xCC; 8], &[0xDD; 8]]),
        ],
    };

    write_json(out_dir().join("limits.json"), &file);
}

fn gen_relative() {
    const MCL: usize = 4096;
    const MCC: usize = 4096;
    const MPL: usize = 4096;

    let make = |name: &str, reference_slices: &[&[u8]], target_slices: &[&[u8]]| -> RelativeCase {
        let reference = make_path::<MCL, MCC, MPL>(reference_slices).expect("valid reference");
        let target = make_path::<MCL, MCC, MPL>(target_slices).expect("valid target");
        RelativeCase {
            name: name.to_string(),
            reference_components_hex: components_to_hex(&reference),
            target_components_hex: components_to_hex(&target),
            encoded_hex: hex::encode(encode_relative(&target, &reference)),
        }
    };

    let file = RelativeFile {
        params: Params { mcl: MCL, mcc: MCC, mpl: MPL },
        cases: vec![
            make("identical", &[b"a", b"b"], &[b"a", b"b"]),
            make("target_extends_reference", &[b"a"], &[b"a", b"b", b"c"]),
            make("reference_extends_target", &[b"a", b"b", b"c"], &[b"a"]),
            make("shared_prefix_then_diverge", &[b"a", b"b", b"c"], &[b"a", b"b", b"d"]),
            make("no_shared_prefix", &[b"x"], &[b"y", b"z"]),
            make("reference_empty", &[], &[b"a"]),
            make("target_empty", &[b"a"], &[]),
            make(
                "shared_prefix_multi_component_suffix",
                &[b"shared1", b"shared2"],
                &[b"shared1", b"shared2", b"suffix1", b"suffix2"],
            ),
            make(
                "shared_prefix_one_diverging_component_with_zero_byte",
                &[b"common", b"alpha"],
                &[b"common", b"\x00\x00\xff"],
            ),
            make("both_empty", &[], &[]),
        ],
    };

    write_json(out_dir().join("relative.json"), &file);
}

fn main() {
    fs::create_dir_all(out_dir()).expect("ensure paths/");
    gen_basic();
    gen_limits();
    gen_relative();
    println!("done.");
}
