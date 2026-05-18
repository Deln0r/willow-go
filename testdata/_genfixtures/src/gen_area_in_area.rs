//! Generate fixtures for encode_area_in_area: an Area encoded relative to
//! a containing reference Area.

mod bytes32;

use std::fs;
use std::path::PathBuf;

use serde::Serialize;
use ufotofu::IntoConsumer;
use ufotofu::codec_relative::RelativeEncodable;
use willow_data_model::groupings::{Area, WillowRange};
use willow_data_model::prelude::Path;

use bytes32::Bytes32;

const MCL: usize = 4096;
const MCC: usize = 4096;
const MPL: usize = 4096;
const ID_WIDTH: usize = 32;

#[derive(Serialize)]
struct Params {
    mcl: usize,
    mcc: usize,
    mpl: usize,
    subspace_id_width: usize,
}

#[derive(Serialize, Clone)]
struct AreaJson {
    /// null means "any subspace"
    subspace_hex: Option<String>,
    path_components_hex: Vec<String>,
    times_start: u64,
    /// null means "open range (no end)"
    times_end: Option<u64>,
}

#[derive(Serialize)]
struct Case {
    name: String,
    rel: AreaJson,
    target: AreaJson,
    encoded_hex: String,
}

#[derive(Serialize)]
struct File {
    params: Params,
    cases: Vec<Case>,
}

fn make_path(slices: &[&[u8]]) -> Path<MCL, MCC, MPL> {
    Path::<MCL, MCC, MPL>::from_slices(slices).expect("valid path")
}

fn make_area(
    subspace: Option<[u8; 32]>,
    path_slices: &[&[u8]],
    times_start: u64,
    times_end: Option<u64>,
) -> Area<MCL, MCC, MPL, Bytes32> {
    let times = match times_end {
        None => WillowRange::new_open(times_start.into()),
        Some(end) => WillowRange::new_closed(times_start.into(), end.into()),
    };
    Area::new(subspace.map(Bytes32), make_path(path_slices), times)
}

fn area_to_json(a: &Area<MCL, MCC, MPL, Bytes32>) -> AreaJson {
    AreaJson {
        subspace_hex: a.subspace().map(|s| hex::encode(s.0)),
        path_components_hex: a.path().components().map(|c| hex::encode(c.as_ref())).collect(),
        times_start: u64::from(*a.times().start()),
        times_end: a.times().end().map(|t| u64::from(*t)),
    }
}

fn encode_area_in_area(
    target: &Area<MCL, MCC, MPL, Bytes32>,
    rel: &Area<MCL, MCC, MPL, Bytes32>,
) -> Vec<u8> {
    let mut buf: Vec<u8> = Vec::new();
    {
        let mut consumer = (&mut buf).into_consumer();
        pollster::block_on(target.relative_encode(rel, &mut consumer))
            .expect("IntoVec encode infallible");
    }
    buf
}

fn make_case(
    name: &str,
    rel: Area<MCL, MCC, MPL, Bytes32>,
    target: Area<MCL, MCC, MPL, Bytes32>,
) -> Case {
    let encoded = encode_area_in_area(&target, &rel);
    Case {
        name: name.to_string(),
        rel: area_to_json(&rel),
        target: area_to_json(&target),
        encoded_hex: hex::encode(encoded),
    }
}

fn out_dir() -> PathBuf {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir.parent().expect("testdata/").join("areas")
}

fn main() {
    let zeros = [0u8; 32];
    let ones = [0x11u8; 32];

    let cases = vec![
        make_case(
            "full_in_full",
            make_area(None, &[], 0, None),
            make_area(None, &[], 0, None),
        ),
        make_case(
            "narrow_subspace_no_rel_subspace",
            make_area(None, &[b"folder"], 100, Some(1000)),
            make_area(Some(ones), &[b"folder", b"file"], 200, Some(500)),
        ),
        make_case(
            "concrete_subspace_in_rel_with_same_subspace",
            make_area(Some(zeros), &[], 0, None),
            make_area(Some(zeros), &[b"a"], 10, Some(100)),
        ),
        make_case(
            "rel_open_target_open_same_start",
            make_area(None, &[], 100, None),
            make_area(None, &[b"x"], 100, None),
        ),
        make_case(
            "rel_open_target_closed",
            make_area(None, &[], 0, None),
            make_area(None, &[], 50, Some(150)),
        ),
        make_case(
            "rel_closed_start_from_end",
            // rel.times = [0, 1000), target.times = [800, 950)
            // start_diff via start = 800, via end = 200 -> end wins
            make_area(None, &[], 0, Some(1000)),
            make_area(None, &[], 800, Some(950)),
        ),
        make_case(
            "rel_closed_end_from_end",
            // rel.times = [0, 1000), target.times = [10, 950)
            // start_diff: start=10 vs end=990, start wins
            // end_diff: start=950 vs end=50, end wins
            make_area(None, &[], 0, Some(1000)),
            make_area(None, &[], 10, Some(950)),
        ),
        make_case(
            "path_extension_only",
            make_area(None, &[b"folder"], 0, None),
            make_area(None, &[b"folder", b"deep", b"file.txt"], 0, None),
        ),
    ];

    let file = File {
        params: Params {
            mcl: MCL,
            mcc: MCC,
            mpl: MPL,
            subspace_id_width: ID_WIDTH,
        },
        cases,
    };

    fs::create_dir_all(out_dir()).expect("ensure areas/");
    let target = out_dir().join("relative.json");
    let json = serde_json::to_string_pretty(&file).expect("serialize");
    fs::write(&target, format!("{json}\n")).expect("write fixture");
    println!("wrote {}", target.display());
}
