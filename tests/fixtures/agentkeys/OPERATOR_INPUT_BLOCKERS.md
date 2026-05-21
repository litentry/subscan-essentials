# AgentKeys Operator Input Blockers

This PR implements decoder, typed-row, and root-row plumbing, but the following issue #12 closing artifacts require operator-supplied inputs that are not present in this repository snapshot.

## Heima V2 Capture / Bulk Replay

Blocked on the canonical `CredentialAudit` V2 deployment address and captured Heima Mainnet logs from the operator demo runs.

Required files once supplied:

- `tests/fixtures/heima-mainnet-canonical-demos.jsonl`
- `tests/fixtures/mainnet-bulk-replay.jsonl`

Each row must include the V2 transaction hash, block number, log index, indexed topics, envelope hash, worker fetch status, and decoded typed body from the indexer.

## Rust Canonical Vectors

Blocked on reference Rust canonical CBOR vector files.

Required files once supplied:

- `tests/fixtures/cross-language-vectors/<op_kind>.json`

Each vector must include `envelope_json`, `canonical_cbor_hex`, and `envelope_hash_hex` so the Go decoder can verify byte-for-byte deterministic CBOR compatibility.
