import React, { useEffect, useState } from 'react'
import { useRouter } from 'next/compat/router'

import { AgentKeysContractLoader } from '@/components/agentkeys'
import { Container, PageContent } from '@/ui'
import { routeSegment } from '@/utils/agentkeys'

export default function Page() {
  const router = useRouter()
  const [id, setId] = useState<string | undefined>()
  useEffect(() => {
    setId((router?.query.id as string | undefined) || routeSegment(router?.asPath, 1))
  }, [router?.asPath, router?.query.id])

  return (
    <PageContent>
      <Container>
        <AgentKeysContractLoader address={id} view="call" />
      </Container>
    </PageContent>
  )
}
