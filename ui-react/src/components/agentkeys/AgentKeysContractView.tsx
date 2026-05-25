import React, { useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  CardBody,
  Divider,
  Input,
  Select,
  SelectItem,
  Spinner,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
  Tabs,
} from '@heroui/react'
import { useRouter } from 'next/compat/router'

import { Link } from '@/components/link'
import { LoadingSpinner, LoadingText } from '@/components/loading'
import { TxTable } from '@/components/tx'
import { agentKeysContractInfoType, agentKeysEventLogType, agentKeysEventType, unwrap, useAgentKeysContract, useAgentKeysEvents } from '@/utils/api'
import {
  AGENT_KEYS_READ_ABI,
  AGENT_KEYS_RPC_URL,
  contractPath,
  decodedEntries,
  formatDecodedValue,
  formatHexNumber,
  routeQueryParam,
} from '@/utils/agentkeys'
import { getThemeColor } from '@/utils/text'
import { env } from 'next-runtime-env'

type ContractTabsProps = {
  address: string
  active: string
  children: React.ReactNode
}

const ContractTabs: React.FC<ContractTabsProps> = ({ address, active, children }) => {
  const router = useRouter()

  return (
    <Card>
      <CardBody>
        <Tabs
          aria-label="AgentKeys contract tabs"
          selectedKey={active}
          variant="underlined"
          color={getThemeColor()}
          onSelectionChange={(key) => {
            if (key === 'overview') router?.push(contractPath(address))
            if (key === 'transactions') router?.push(contractPath(address, 'txs'))
            if (key === 'events') router?.push(contractPath(address, 'events'))
            if (key === 'call') router?.push(contractPath(address, 'call'))
          }}>
          <Tab key="overview" title="Overview" />
          <Tab key="transactions" title="Transactions" />
          <Tab key="events" title="Events" />
          <Tab key="call" title="Read Functions" />
        </Tabs>
        <div className="pt-4">{children}</div>
      </CardBody>
    </Card>
  )
}

const DetailRow = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <>
    <div className="flex flex-col gap-1 py-1 sm:flex-row sm:items-start">
      <div className="w-48 shrink-0 text-sm text-gray-500">{label}</div>
      <div className="min-w-0 flex-1 break-all text-sm sm:text-base">{children}</div>
    </div>
    <Divider className="my-2.5" />
  </>
)

const FunctionList = ({ title, functions }: { title: string; functions: string[] }) => (
  <div>
    <div className="mb-2 text-sm font-semibold">{title}</div>
    <div className="flex flex-wrap gap-2">
      {functions.map((item) => (
        <span key={item} className="rounded-md border border-default-200 px-2 py-1 font-mono text-xs">
          {item}
        </span>
      ))}
    </div>
  </div>
)

export const AgentKeysOverview: React.FC<{ address: string; data: agentKeysContractInfoType }> = ({ address, data }) => {
  const { contract, indexed_record: indexedRecord } = data

  return (
    <>
      <div className="flex flex-col gap-1 lg:flex-row">
        <div className="text-base">AgentKeys Contract</div>
        <div className="break-all text-sm sm:text-base">#{contract.address}</div>
      </div>
      <Card>
        <CardBody>
          <DetailRow label="Name">{contract.name}</DetailRow>
          <DetailRow label="Chain ID">{contract.chain_id}</DetailRow>
          <DetailRow label="Bytecode Size">{contract.bytecode_size} bytes</DetailRow>
          <DetailRow label="Deploy Date">{contract.deploy_date}</DetailRow>
          <DetailRow label="Compiler">{contract.compiler}</DetailRow>
          <DetailRow label="EVM Version">{contract.evm_version}</DetailRow>
          <DetailRow label="Deploy Tx">
            {indexedRecord?.tx_hash ? <Link href={`/tx/${indexedRecord.tx_hash}`}>{indexedRecord.tx_hash}</Link> : 'Waiting for indexed deploy tx'}
          </DetailRow>
          <DetailRow label="Function Call Counter">{indexedRecord?.transaction_count ?? 0}</DetailRow>
          <DetailRow label="Indexed Record">{indexedRecord ? 'Present in DB' : 'Not indexed yet'}</DetailRow>
          <div className="grid gap-4 lg:grid-cols-2">
            <FunctionList title="Read Functions" functions={contract.read_functions} />
            <FunctionList title="Write Functions" functions={contract.write_functions} />
          </div>
        </CardBody>
      </Card>
      <ContractTabs address={address} active="overview">
        <div className="grid gap-3 lg:grid-cols-2">
          {data.events.map((item) => (
            <div key={`${item.address}-${item.topic0}`} className="rounded-md border border-default-200 p-3">
              <div className="font-semibold">{item.name}</div>
              <div className="break-all font-mono text-xs text-gray-500">{item.signature}</div>
              <Link href={`${contractPath(item.address, 'events')}?topic0=${item.topic0}`}>Open filtered events</Link>
            </div>
          ))}
        </div>
      </ContractTabs>
    </>
  )
}

export const AgentKeysTransactions: React.FC<{ address: string }> = ({ address }) => (
  <ContractTabs address={address} active="transactions">
    <TxTable args={{ address }} />
  </ContractTabs>
)

const EventArgs: React.FC<{ log: agentKeysEventLogType }> = ({ log }) => {
  const entries = decodedEntries(log)
  if (entries.length === 0) return <span>-</span>
  return (
    <div className="flex flex-col gap-1">
      {entries.map((entry) => (
        <div key={entry.name} className="break-all">
          <span className="font-semibold">{entry.name}</span>: <span className="font-mono text-xs">{entry.value}</span>
        </div>
      ))}
    </div>
  )
}

export const AgentKeysEvents: React.FC<{ address: string; events?: agentKeysEventType[] }> = ({ address }) => {
  const router = useRouter()
  const NEXT_PUBLIC_API_HOST = env('NEXT_PUBLIC_API_HOST') || ''
  const topic0 = (typeof router?.query.topic0 === 'string' ? router.query.topic0 : undefined) || routeQueryParam(router?.asPath, 'topic0')
  const { data, isLoading } = useAgentKeysEvents(NEXT_PUBLIC_API_HOST, { address, topic0, row: 50 })
  const eventData = unwrap(data)
  const items = eventData?.list || []

  return (
    <ContractTabs address={address} active="events">
      <div className="mb-3 flex flex-col gap-1">
        <div className="text-sm text-gray-500">Topic filter</div>
        <div className="break-all font-mono text-xs">{topic0 || 'All AgentKeys events for this contract'}</div>
      </div>
      <Table
        aria-label="AgentKeys events"
        classNames={{
          wrapper: 'min-h-[222px]',
          td: 'align-top',
        }}>
        <TableHeader>
          <TableColumn key="event">Event</TableColumn>
          <TableColumn key="block">Block</TableColumn>
          <TableColumn key="tx">Transaction</TableColumn>
          <TableColumn key="topic0">Topic[0]</TableColumn>
          <TableColumn key="decoded">Indexed Args</TableColumn>
        </TableHeader>
        <TableBody
          isLoading={isLoading}
          loadingContent={<Spinner color={getThemeColor()} />}
          items={items}
          emptyContent="No indexed AgentKeys events">
          {(item) => (
            <TableRow key={`${item.transactionHash}-${item.logIndex}`}>
              <TableCell>
                <div className="font-semibold">{item.event_name}</div>
                <div className="break-all font-mono text-xs text-gray-500">{item.event_signature}</div>
              </TableCell>
              <TableCell>
                <Link href={`/block/${formatHexNumber(item.blockNumber)}`}>{formatHexNumber(item.blockNumber)}</Link>
              </TableCell>
              <TableCell>
                <Link href={`/tx/${item.transactionHash}`}>{item.transactionHash}</Link>
              </TableCell>
              <TableCell>
                <span className="break-all font-mono text-xs">{item.topic0}</span>
              </TableCell>
              <TableCell>
                <EventArgs log={item} />
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </ContractTabs>
  )
}

export const AgentKeysCall: React.FC<{ address: string; data: agentKeysContractInfoType }> = ({ address, data }) => {
  const callableFunctions = useMemo(() => data.contract.read_functions.filter((item) => AGENT_KEYS_READ_ABI[item]), [data.contract.read_functions])
  const preferredFunction = callableFunctions.find((item) => item === 'isActive(bytes32)') || callableFunctions[0] || ''
  const [selectedFunction, setSelectedFunction] = useState(preferredFunction)
  const [args, setArgs] = useState<Record<string, string>>({})
  const [result, setResult] = useState<{ name: string; value: string }[] | null>(null)
  const [rawResult, setRawResult] = useState('')
  const [error, setError] = useState('')
  const [isCalling, setIsCalling] = useState(false)
  const abi = AGENT_KEYS_READ_ABI[selectedFunction]

  useEffect(() => {
    setSelectedFunction(preferredFunction)
  }, [preferredFunction])

  const callFunction = async () => {
    if (!abi) return
    setIsCalling(true)
    setError('')
    setResult(null)
    setRawResult('')
    try {
      const Web3Module = await import('web3')
      const Web3Ctor = (Web3Module as any).default || (Web3Module as any).Web3
      const web3 = new Web3Ctor(AGENT_KEYS_RPC_URL)
      const values = abi.inputs.map((input) => args[input.name] || '')
      const callData = web3.eth.abi.encodeFunctionCall(abi as any, values)
      const callResult = await web3.eth.call({ to: data.contract.address, data: callData })
      const decoded = web3.eth.abi.decodeParameters(abi.outputs, callResult)
      setRawResult(callResult)
      setResult(
        abi.outputs.map((output, index) => ({
          name: output.name || `result${index}`,
          value: formatDecodedValue(decoded[index]),
        }))
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setIsCalling(false)
    }
  }

  return (
    <ContractTabs address={address} active="call">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,420px)_1fr]">
        <div className="flex flex-col gap-3">
          <Select
            label="Read function"
            selectedKeys={selectedFunction ? [selectedFunction] : []}
            onSelectionChange={(keys) => {
              const next = Array.from(keys as Set<string>)[0] || ''
              setSelectedFunction(next)
              setArgs({})
              setResult(null)
              setRawResult('')
              setError('')
            }}>
            {callableFunctions.map((item) => (
              <SelectItem key={item}>{item}</SelectItem>
            ))}
          </Select>
          {abi?.inputs.map((input) => (
            <Input
              key={input.name}
              label={`${input.name} (${input.type})`}
              value={args[input.name] || ''}
              onValueChange={(value) => setArgs((prev) => ({ ...prev, [input.name]: value }))}
              placeholder={input.type.startsWith('bytes32') ? '0x...' : 'value'}
            />
          ))}
          <Button color={getThemeColor()} isLoading={isCalling} onPress={callFunction}>
            Call
          </Button>
          <div className="break-all text-xs text-gray-500">RPC: {AGENT_KEYS_RPC_URL}</div>
        </div>
        <Card>
          <CardBody>
            <div className="mb-2 font-semibold">Decoded Result</div>
            {error && <div className="break-all text-sm text-danger">{error}</div>}
            {!error && !result && <div className="text-sm text-gray-500">Run a read-only call to see decoded output.</div>}
            {result?.map((item) => (
              <DetailRow key={item.name} label={item.name}>
                <span className="font-mono text-xs">{item.value}</span>
              </DetailRow>
            ))}
            {rawResult && (
              <div className="mt-3">
                <div className="mb-1 text-sm text-gray-500">Raw eth_call result</div>
                <div className="break-all font-mono text-xs">{rawResult}</div>
              </div>
            )}
          </CardBody>
        </Card>
      </div>
    </ContractTabs>
  )
}

export const AgentKeysContractLoader: React.FC<{ address?: string; view: 'overview' | 'transactions' | 'events' | 'call' }> = ({ address, view }) => {
  const NEXT_PUBLIC_API_HOST = env('NEXT_PUBLIC_API_HOST') || ''
  const { data, isLoading } = useAgentKeysContract(NEXT_PUBLIC_API_HOST, address ? { address } : null)
  const contractData = unwrap(data)

  if (isLoading) return <LoadingSpinner />
  if (!address || !contractData) return <LoadingText />
  if (view === 'transactions') return <AgentKeysTransactions address={address} />
  if (view === 'events') return <AgentKeysEvents address={address} events={contractData.events} />
  if (view === 'call') return <AgentKeysCall address={address} data={contractData} />
  return <AgentKeysOverview address={address} data={contractData} />
}
