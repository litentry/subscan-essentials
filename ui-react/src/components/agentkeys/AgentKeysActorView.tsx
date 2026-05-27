import React from 'react'
import { Card, CardBody, Spinner, Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from '@heroui/react'

import { Link } from '@/components/link'
import { LoadingSpinner, LoadingText } from '@/components/loading'
import { agentKeysEventLogType, unwrap, useAgentKeysActor } from '@/utils/api'
import { decodedEntries, formatHexNumber } from '@/utils/agentkeys'
import { getThemeColor } from '@/utils/text'
import { env } from 'next-runtime-env'

const Metric = ({ label, value }: { label: string; value: number }) => (
  <Card>
    <CardBody>
      <div className="text-sm text-gray-500">{label}</div>
      <div className="text-2xl font-semibold">{value}</div>
    </CardBody>
  </Card>
)

const EventArgs = ({ log }: { log: agentKeysEventLogType }) => {
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

const EventTable = ({ title, items, isLoading }: { title: string; items: agentKeysEventLogType[]; isLoading?: boolean }) => (
  <Card>
    <CardBody>
      <div className="mb-3 text-base font-semibold">{title}</div>
      <Table aria-label={title} classNames={{ wrapper: 'min-h-[180px]', td: 'align-top' }}>
        <TableHeader>
          <TableColumn key="event">Event</TableColumn>
          <TableColumn key="block">Block</TableColumn>
          <TableColumn key="tx">Transaction</TableColumn>
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
                <EventArgs log={item} />
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </CardBody>
  </Card>
)

export const AgentKeysActorView: React.FC<{ actorOmni?: string }> = ({ actorOmni }) => {
  const NEXT_PUBLIC_API_HOST = env('NEXT_PUBLIC_API_HOST') || ''
  const { data, isLoading } = useAgentKeysActor(NEXT_PUBLIC_API_HOST, actorOmni ? { actor_omni: actorOmni } : null)
  const actorData = unwrap(data)

  if (isLoading) return <LoadingSpinner />
  if (!actorOmni || !actorData) return <LoadingText />

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1 lg:flex-row">
        <div className="text-base">AgentKeys Actor</div>
        <div className="break-all text-sm sm:text-base">#{actorData.actor_omni}</div>
      </div>
      <div className="grid gap-3 md:grid-cols-4">
        <Metric label="Devices Registered" value={actorData.devices_registered} />
        <Metric label="Scope Grants" value={actorData.scope_grants} />
        <Metric label="Audit Entries" value={actorData.audit_entries} />
        <Metric label="Current K3 Epoch" value={actorData.current_k3_epoch} />
      </div>
      <EventTable title="Devices" items={actorData.devices || []} isLoading={isLoading} />
      <EventTable title="Scope Grants" items={actorData.scopes || []} isLoading={isLoading} />
      <EventTable title="Audit Entries" items={actorData.audits || []} isLoading={isLoading} />
    </div>
  )
}
