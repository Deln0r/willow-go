//! Generate WILLIAM3 digest fixtures using the upstream bab_rs v0.5.0
//! batch_hash function. Used to verify the Go port of the algorithm produces
//! byte-identical digests for the same inputs.

use std::fs;
use std::path::PathBuf;

use bab_rs::{batch_hash, William3Digest};
use serde::Serialize;

#[derive(Serialize)]
struct Case {
    name: String,
    input_hex: String,
    digest_hex: String,
}

#[derive(Serialize)]
struct File {
    chunk_size: usize,
    digest_width: usize,
    cases: Vec<Case>,
}

fn william3(input: &[u8]) -> [u8; 32] {
    let mut digest = William3Digest::default();
    batch_hash(input, &mut digest);
    digest.into_bytes()
}

fn make_case(name: &str, input: Vec<u8>) -> Case {
    let digest = william3(&input);
    Case {
        name: name.to_string(),
        input_hex: hex::encode(&input),
        digest_hex: hex::encode(digest),
    }
}

fn out_dir() -> PathBuf {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .parent()
        .expect("testdata/")
        .join("william3")
}

fn main() {
    let cases = vec![
        make_case("empty", vec![]),
        make_case("single_byte_zero", vec![0]),
        make_case("single_byte_ff", vec![0xFF]),
        make_case("hello_world", b"hello world".to_vec()),
        make_case("ascii_64_bytes", b"abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz12".to_vec()),
        make_case("exactly_1023_bytes", vec![0x55; 1023]),
        make_case("exactly_1024_bytes_one_chunk", vec![0x66; 1024]),
        make_case("exactly_1025_bytes_two_chunks", vec![0x77; 1025]),
        make_case("exactly_2048_bytes_two_chunks", vec![0x88; 2048]),
        make_case("exactly_3000_bytes_three_chunks", vec![0xAA; 3000]),
        make_case("nonzero_pattern_5000_bytes", {
            let mut v = vec![0; 5000];
            for (i, b) in v.iter_mut().enumerate() {
                *b = (i % 251) as u8;
            }
            v
        }),
    ];

    let file = File {
        chunk_size: 1024,
        digest_width: 32,
        cases,
    };

    fs::create_dir_all(out_dir()).expect("ensure william3/");
    let target = out_dir().join("digests.json");
    let json = serde_json::to_string_pretty(&file).expect("serialize");
    fs::write(&target, format!("{json}\n")).expect("write fixture");
    println!("wrote {}", target.display());
}
