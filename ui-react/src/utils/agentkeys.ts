import { agentKeysContractType, agentKeysEventLogType } from './api'

export const AGENT_KEYS_RPC_URL = 'https://rpc.heima-parachain.heima.network'

type AbiInput = {
  name: string
  type: string
  components?: AbiInput[]
}

export type AgentKeysFunctionAbi = {
  type: 'function'
  name: string
  stateMutability: 'view' | 'nonpayable'
  inputs: AbiInput[]
  outputs: AbiInput[]
}

const deviceEntryComponents: AbiInput[] = [
  { name: 'operatorOmni', type: 'bytes32' },
  { name: 'actorOmni', type: 'bytes32' },
  { name: 'k11CredId', type: 'bytes32' },
  { name: 'k11RpIdHash', type: 'bytes32' },
  { name: 'k11PubX', type: 'uint256' },
  { name: 'k11PubY', type: 'uint256' },
  { name: 'tier', type: 'uint8' },
  { name: 'roles', type: 'uint8' },
  { name: 'registeredAt', type: 'uint64' },
  { name: 'lastSignCount', type: 'uint32' },
  { name: 'revoked', type: 'bool' },
]

const scopeComponents: AbiInput[] = [
  { name: 'services', type: 'bytes32[]' },
  { name: 'readOnly', type: 'bool' },
  { name: 'maxPerCall', type: 'uint128' },
  { name: 'maxPerPeriod', type: 'uint128' },
  { name: 'maxTotal', type: 'uint128' },
  { name: 'periodSeconds', type: 'uint32' },
  { name: 'updatedAt', type: 'uint64' },
  { name: 'exists', type: 'bool' },
]

export const AGENT_KEYS_READ_ABI: Record<string, AgentKeysFunctionAbi> = {
  'registry()': {
    type: 'function',
    name: 'registry',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'registry', type: 'address' }],
  },
  'getScope(bytes32,bytes32)': {
    type: 'function',
    name: 'getScope',
    stateMutability: 'view',
    inputs: [
      { name: 'operatorOmni', type: 'bytes32' },
      { name: 'agentOmni', type: 'bytes32' },
    ],
    outputs: [{ name: 'scope', type: 'tuple', components: scopeComponents }],
  },
  'isServiceInScope(bytes32,bytes32,bytes32)': {
    type: 'function',
    name: 'isServiceInScope',
    stateMutability: 'view',
    inputs: [
      { name: 'operatorOmni', type: 'bytes32' },
      { name: 'agentOmni', type: 'bytes32' },
      { name: 'serviceHash', type: 'bytes32' },
    ],
    outputs: [{ name: 'inScope', type: 'bool' }],
  },
  'ROLE_CAP_MINT()': {
    type: 'function',
    name: 'ROLE_CAP_MINT',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'role', type: 'uint8' }],
  },
  'ROLE_RECOVERY()': {
    type: 'function',
    name: 'ROLE_RECOVERY',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'role', type: 'uint8' }],
  },
  'ROLE_SCOPE_MGMT()': {
    type: 'function',
    name: 'ROLE_SCOPE_MGMT',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'role', type: 'uint8' }],
  },
  'TIER_MASTER()': {
    type: 'function',
    name: 'TIER_MASTER',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'tier', type: 'uint8' }],
  },
  'TIER_AGENT()': {
    type: 'function',
    name: 'TIER_AGENT',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'tier', type: 'uint8' }],
  },
  'devices(bytes32)': {
    type: 'function',
    name: 'devices',
    stateMutability: 'view',
    inputs: [{ name: 'deviceKeyHash', type: 'bytes32' }],
    outputs: deviceEntryComponents,
  },
  'getDevice(bytes32)': {
    type: 'function',
    name: 'getDevice',
    stateMutability: 'view',
    inputs: [{ name: 'deviceKeyHash', type: 'bytes32' }],
    outputs: [{ name: 'device', type: 'tuple', components: deviceEntryComponents }],
  },
  'getOperatorDevices(bytes32)': {
    type: 'function',
    name: 'getOperatorDevices',
    stateMutability: 'view',
    inputs: [{ name: 'operatorOmni', type: 'bytes32' }],
    outputs: [{ name: 'deviceKeyHashes', type: 'bytes32[]' }],
  },
  'isActive(bytes32)': {
    type: 'function',
    name: 'isActive',
    stateMutability: 'view',
    inputs: [{ name: 'deviceKeyHash', type: 'bytes32' }],
    outputs: [{ name: 'active', type: 'bool' }],
  },
  'operatorMasterWallet(bytes32)': {
    type: 'function',
    name: 'operatorMasterWallet',
    stateMutability: 'view',
    inputs: [{ name: 'operatorOmni', type: 'bytes32' }],
    outputs: [{ name: 'masterWallet', type: 'address' }],
  },
  'currentEpoch()': {
    type: 'function',
    name: 'currentEpoch',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'epoch', type: 'uint256' }],
  },
  'signerGovernance()': {
    type: 'function',
    name: 'signerGovernance',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'governance', type: 'address' }],
  },
  'epochStartedAt(uint256)': {
    type: 'function',
    name: 'epochStartedAt',
    stateMutability: 'view',
    inputs: [{ name: 'epoch', type: 'uint256' }],
    outputs: [{ name: 'startedAt', type: 'uint256' }],
  },
  'OP_STORE()': {
    type: 'function',
    name: 'OP_STORE',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'opType', type: 'uint8' }],
  },
  'OP_READ()': {
    type: 'function',
    name: 'OP_READ',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'opType', type: 'uint8' }],
  },
  'OP_TEARDOWN()': {
    type: 'function',
    name: 'OP_TEARDOWN',
    stateMutability: 'view',
    inputs: [],
    outputs: [{ name: 'opType', type: 'uint8' }],
  },
  'getEntries(bytes32,uint256,uint256)': {
    type: 'function',
    name: 'getEntries',
    stateMutability: 'view',
    inputs: [
      { name: 'operatorOmni', type: 'bytes32' },
      { name: 'offset', type: 'uint256' },
      { name: 'limit', type: 'uint256' },
    ],
    outputs: [
      {
        name: 'page',
        type: 'tuple[]',
        components: [
          { name: 'actorOmni', type: 'bytes32' },
          { name: 'serviceHash', type: 'bytes32' },
          { name: 'payloadHash', type: 'bytes32' },
          { name: 'timestamp', type: 'uint64' },
          { name: 'opType', type: 'uint8' },
        ],
      },
    ],
  },
  'entryCount(bytes32)': {
    type: 'function',
    name: 'entryCount',
    stateMutability: 'view',
    inputs: [{ name: 'operatorOmni', type: 'bytes32' }],
    outputs: [{ name: 'count', type: 'uint256' }],
  },
}

const AGENT_KEYS_WRITE_ABI: AgentKeysFunctionAbi[] = [
  {
    type: 'function',
    name: 'registerMasterDevice',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'deviceKeyHash', type: 'bytes32' },
      { name: 'operatorOmni', type: 'bytes32' },
      { name: 'actorOmni', type: 'bytes32' },
      { name: 'k11CredId', type: 'bytes32' },
      { name: 'attestation', type: 'bytes' },
      { name: 'roles', type: 'uint8' },
      { name: 'k11PopSig', type: 'bytes' },
    ],
    outputs: [],
  },
  {
    type: 'function',
    name: 'registerAgentDevice',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'deviceKeyHash', type: 'bytes32' },
      { name: 'operatorOmni', type: 'bytes32' },
      { name: 'actorOmni', type: 'bytes32' },
      { name: 'linkCodeRedemption', type: 'bytes' },
      { name: 'agentPopSig', type: 'bytes' },
    ],
    outputs: [],
  },
  {
    type: 'function',
    name: 'setScopeWithWebauthn',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'operatorOmni', type: 'bytes32' },
      { name: 'agentOmni', type: 'bytes32' },
      { name: 'services', type: 'bytes32[]' },
      { name: 'readOnly', type: 'bool' },
      { name: 'maxPerCall', type: 'uint128' },
      { name: 'maxPerPeriod', type: 'uint128' },
      { name: 'maxTotal', type: 'uint128' },
      { name: 'periodSeconds', type: 'uint32' },
      { name: 'assertion', type: 'bytes' },
    ],
    outputs: [],
  },
  {
    type: 'function',
    name: 'append',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'operatorOmni', type: 'bytes32' },
      { name: 'actorOmni', type: 'bytes32' },
      { name: 'serviceHash', type: 'bytes32' },
      { name: 'opType', type: 'uint8' },
      { name: 'payloadHash', type: 'bytes32' },
    ],
    outputs: [],
  },
]

export const contractPath = (address: string, tab?: 'txs' | 'events' | 'call') => {
  const base = `/contract/${address}`
  if (tab === 'events') return `${base}/events`
  if (tab === 'call') return `${base}/call`
  return base
}

export const routeSegment = (asPath: string | undefined, indexFromEnd = 0) => {
  const browserPath = typeof window !== 'undefined' ? `${window.location.pathname}${window.location.search}` : ''
  const source = !asPath || asPath.includes('[') ? browserPath : asPath
  const path = source.split('?')[0]
  const parts = path.split('/').filter(Boolean)
  return parts[parts.length - 1 - indexFromEnd]
}

export const routeQueryParam = (asPath: string | undefined, key: string) => {
  const browserPath = typeof window !== 'undefined' ? `${window.location.pathname}${window.location.search}` : ''
  const source = !asPath || asPath.includes('[') ? browserPath : asPath
  const query = source.split('?')[1]
  if (!query) return undefined
  return new URLSearchParams(query).get(key) || undefined
}

export const isAgentKeysContract = (contract?: agentKeysContractType | null) => {
  return Boolean(contract?.chain_id === 212013 && contract?.read_functions)
}

export const formatHexNumber = (value?: string) => {
  if (!value) return '-'
  if (!value.startsWith('0x')) return value
  const parsed = Number.parseInt(value, 16)
  return Number.isFinite(parsed) ? parsed.toString() : value
}

export const formatDecodedValue = (value: unknown): string => {
  if (value === null || value === undefined) return '-'
  if (typeof value === 'bigint') return value.toString()
  if (Array.isArray(value)) return `[${value.map(formatDecodedValue).join(', ')}]`
  if (typeof value === 'object') {
    return JSON.stringify(value, (_key, item) => (typeof item === 'bigint' ? item.toString() : item), 2)
  }
  return String(value)
}

export const decodedEntries = (log: agentKeysEventLogType) => {
  return Object.entries(log.decoded || {}).map(([name, value]) => ({ name, value: formatDecodedValue(value) }))
}

export const findAgentKeysWriteFunction = async (inputData?: string) => {
  if (!inputData || inputData.length < 10) return null
  const Web3Module = await import('web3')
  const Web3Ctor = (Web3Module as any).default || (Web3Module as any).Web3
  const web3 = new Web3Ctor()
  const selector = inputData.slice(0, 10).toLowerCase()

  for (const item of AGENT_KEYS_WRITE_ABI) {
    if (web3.eth.abi.encodeFunctionSignature(item).toLowerCase() !== selector) continue
    const decoded = web3.eth.abi.decodeParameters(item.inputs, `0x${inputData.slice(10)}`)
    return {
      name: item.name,
      args: item.inputs.map((input, index) => ({
        name: input.name || `arg${index}`,
        value: formatDecodedValue(decoded[index]),
      })),
    }
  }
  return null
}
