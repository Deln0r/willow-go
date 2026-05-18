//! Generate fixtures for the path_extends_path encoding (a path encoded
//! relative to one of its prefixes, emitting only the suffix as an absolute
//! path encoding).

use std::fs;
use std::path::PathBuf;

use serde::Serialize;
use ufotofu::IntoConsumer;
use willow_data_model::paths::path_extends_path::encode_path_extends_path;
use willow_data_model::prelude::Path;

#[derive(Serialize)]
struct Params {
    mcl: usize,
    mcc: usize,
    mpl: usize,
}

#[derive(Serialize)]
struct Case {
    name: String,
    prefix_components_hex: Vec<String>,
    path_components_hex: Vec<String>,
    encoded_hex: String,
}

#[derive(Serialize)]
struct File {
    params: Params,
    cases: Vec<Case>,
}

const MCL: usize = 4096;
const MCC: usize = 4096;
const MPL: usize = 4096;

fn make_path(slices: &[&[u8]]) -> Path<MCL, MCC, MPL> {
    Path::<MCL, MCC, MPL>::from_slices(slices).expect("valid path")
}

fn encode_extending(path: &Path<MCL, MCC, MPL>, prefix: &Path<MCL, MCC, MPL>) -> Vec<u8> {
    let mut buf: Vec<u8> = Vec::new();
    {
        let mut consumer = (&mut buf).into_consumer();
        pollster::block_on(encode_path_extends_path::<MCL, MCC, MPL, _>(
            path,
            prefix,
            &mut consumer,
        ))
        .expect("IntoVec encode infallible");
    }
    buf
}

fn make_case(name: &str, prefix_slices: &[&[u8]], path_slices: &[&[u8]]) -> Case {
    let prefix = make_path(prefix_slices);
    let path = make_path(path_slices);
    let encoded = encode_extending(&path, &prefix);
    Case {
        name: name.to_string(),
        prefix_components_hex: prefix
            .components()
            .map(|c| hex::encode(c.as_ref()))
            .collect(),
        path_components_hex: path.components().map(|c| hex::encode(c.as_ref())).collect(),
        encoded_hex: hex::encode(encoded),
    }
}

fn out_dir() -> PathBuf {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .parent()
        .expect("testdata/")
        .join("paths_rel")
}

fn main() {
    let cases = vec![
        make_case("empty_prefix_empty_path", &[], &[]),
        make_case("empty_prefix_single_component", &[], &[b"alfie"]),
        make_case("equal_prefix_and_path", &[b"folder"], &[b"folder"]),
        make_case(
            "single_component_prefix_extends_by_one",
            &[b"folder"],
            &[b"folder", b"file"],
        ),
        make_case(
            "single_component_prefix_extends_by_two",
            &[b"folder"],
            &[b"folder", b"sub", b"file.txt"],
        ),
        make_case(
            "two_component_prefix_extends_by_one",
            &[b"a", b"b"],
            &[b"a", b"b", b"c"],
        ),
        make_case(
            "prefix_extends_with_zero_bytes_in_suffix",
            &[b"namespace"],
            &[b"namespace", b"\x00\x00\x01", b"file"],
        ),
    ];

    let file = File {
        params: Params {
            mcl: MCL,
            mcc: MCC,
            mpl: MPL,
        },
        cases,
    };

    fs::create_dir_all(out_dir()).expect("ensure paths_rel/");
    let target = out_dir().join("extends.json");
    let json = serde_json::to_string_pretty(&file).expect("serialize");
    fs::write(&target, format!("{json}\n")).expect("write fixture");
    println!("wrote {}", target.display());
}
