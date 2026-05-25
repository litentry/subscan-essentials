import React, { useEffect, useState } from 'react'
import { useRouter } from 'next/compat/router'

import { AgentKeysActorView } from '@/components/agentkeys'
import { Container, PageContent } from '@/ui'
import { routeSegment } from '@/utils/agentkeys'

export default function Page() {
  const router = useRouter()
  const [actor, setActor] = useState<string | undefined>()
  useEffect(() => {
    setActor((router?.query.actor as string | undefined) || routeSegment(router?.asPath))
  }, [router?.asPath, router?.query.actor])

  return (
    <PageContent>
      <Container>
        <AgentKeysActorView actorOmni={actor} />
      </Container>
    </PageContent>
  )
}
