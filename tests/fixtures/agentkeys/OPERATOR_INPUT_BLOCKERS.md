# AgentKeys Operator Input Blockers

This PR implements decoder, typed-row, and root-row plumbing, but issue #14
requires a real Heima Mainnet delivery loop. Issue #12 was scanned for the
`CredentialAudit` V2 deployment inputs and only states that the exact address
will be supplied by the operator alongside the closing PR tx capture. The exact
contract address and deploy block are not present in issue #12, PR #13, or this
repository snapshot.

Do not replace the missing inputs with mock logs or hand-crafted fixture rows.

## Required Heima Mainnet Contract Inputs

Blocked on all of the following operator-supplied values:

- Heima Mainnet chain ID: `212013`
- `CredentialAudit` V2 contract address
- V2 deploy block, or the earliest block height that can be used as the
  `eth_getLogs` lower bound
- Worker base URL if it differs from `https://audit.litentry.org`
- Operator account / operator omni used for the canonical demo and bulk replay
  capture filters

## Required Canonical Demo Capture

Blocked on real `AuditAppendedV2` and `AuditRootAppendedV2` logs for the
foundation, hardening, and isolation demo runs.

The resulting `tests/fixtures/heima-mainnet-canonical-demos.jsonl` rows must be
created from real chain logs and worker responses. Each row must include:

- `demo`
- `txhash`
- `block_number`
- `log_index`
- `topics`
- `op_kind`
- `envelope_hash`
- `envelope_fetched_from_worker`
- `hash_verified` where `keccak256(canonical_cbor_bytes) == envelope_hash`
- decoded typed body as returned through `/agentkeys/audit/...`

## Required Bulk Replay Capture

Blocked on a reproducible mainnet log dump from the canonical contract between
`deploy_block` and the latest block used when the PR evidence is published.

Required file once supplied:

- `tests/fixtures/mainnet-bulk-replay.jsonl`

Each row must include the V2 transaction hash, block number, log index, indexed
topics, `op_kind`, and `envelope_hash`. The replay must prove every real event
in that range can be worker-fetched, hash-verified, decoded, and exposed as a
typed or `Unknown(byte)` row.

## Required Rust Canonical Vectors

Blocked on reference Rust canonical CBOR vector files.

Required files once supplied:

- `tests/fixtures/cross-language-vectors/<op_kind>.json`

Each vector must include `envelope_json`, `canonical_cbor_hex`, and
`envelope_hash_hex` so the Go decoder can verify byte-for-byte deterministic
CBOR compatibility.
