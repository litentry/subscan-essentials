# AgentKeys Operator Input Blockers

This PR implements decoder, typed-row, and root-row plumbing, but issue #12
requires a real Heima Mainnet delivery loop. Issue #12 itself says the exact
V2 address is supplied with the closing PR capture, while issues #3 and #4
already list the current AgentKeys stage-1 `CredentialAudit` address:
`0x1801ded1a4FBD8c9224Ab18B9EcbB293B8674c06`.

That address has non-empty bytecode on Heima Mainnet via `eth_getCode`, so the
remaining blocker is not the address. The remaining blockers are the log lower
bound/deploy block for the V2 audit events and the real canonical event/worker
capture artifacts.

Do not replace the missing inputs with mock logs or hand-crafted fixture rows.

## Required Heima Mainnet Contract Inputs

Known contract input:

- Heima Mainnet chain ID: `212013`
- `CredentialAudit`: `0x1801ded1a4FBD8c9224Ab18B9EcbB293B8674c06`
- Source: litentry/subscan-essentials issues #3 and #4

Blocked on the following operator-supplied values:

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
