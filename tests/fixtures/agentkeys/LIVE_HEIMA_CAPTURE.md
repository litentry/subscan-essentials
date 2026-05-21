# AgentKeys Heima Live Capture

Issue #12 comment data identifies the current Heima Mainnet
`CredentialAudit` contract as:

`0x63c4545ac01c77cc74044f25b8edea3880224577`

This fixture set records real logs from that contract and keeps them separate
from hand-crafted unit fixtures.

## Files

- `heima-mainnet-current-auditappended.jsonl` captures live
  `AuditAppended(bytes32,bytes32,bytes32,uint8,uint256,bytes32)` logs from
  Heima Mainnet over the latest-block window used when the fixture was
  generated.

Each JSONL row records the raw topics, raw ABI data, decoded operator/actor,
decoded `op_kind`, decoded current sequence, transaction identity, block
identity, and the worker fetch status for the emitted hash.

## Runtime behavior represented by this capture

- The backend supports the issue #12 V2 event topic:
  `AuditAppendedV2(bytes32,bytes32,uint8,bytes32)`.
- The backend also supports the live Heima current event topic:
  `AuditAppended(bytes32,bytes32,bytes32,uint8,uint256,bytes32)`.
- The live current-event hashes in this capture returned `404` from
  `https://audit.litentry.org/v1/audit/envelope/<hash>` at capture time, so
  the REST row path preserves the chain row with `envelope_available=false`
  instead of failing the whole page.

When the upstream publisher starts emitting worker-backed V2 hashes, the same
strict decoder path verifies `keccak256(canonical_cbor_bytes) == envelope_hash`
and renders typed bodies.
