import { Providers } from '@/components/providers/providers'
import { Footer } from '@/components/Footer'
import { GetServerSidePropsContext } from 'next'
import type { AppProps } from 'next/app'
import Head from 'next/head'
import { useData } from '@/context'
import { env } from 'next-runtime-env'

const MainContainer = ({ children }: { children: React.ReactNode }) => {
  const { metadata } = useData()

  return (
    <div className={`${metadata?.networkNode || 'heima'} heima-shell flex flex-col min-h-screen`}>
      <div className="flex-grow">{children}</div>
      <Footer />
    </div>
  )
}

export default function RootLayout({
  children,
}: {
  context?: GetServerSidePropsContext
  children: React.ReactNode
  pageProps: AppProps
}) {
  const explorerName = env('NEXT_PUBLIC_EXPLORER_NAME') || 'Heima Explorer'

  return (
    <div id="app" className={`font-sans flex min-h-screen flex-col`}>
      <Head>
        <title>{explorerName}</title>
        <meta name="viewport" content="width=device-width,initial-scale=1.0,viewport-fit=cover"></meta>
        <link rel="icon" href="/favicon.ico?v=heima-20260511" type="image/x-icon" sizes="16x16" />
        <link rel="shortcut icon" href="/favicon.ico?v=heima-20260511" type="image/x-icon" />
        <link rel="apple-touch-icon" href="/favicon.png" />
        <script defer src="/__ENV.js" />
      </Head>
      <Providers>
        <MainContainer>{children}</MainContainer>
      </Providers>
    </div>
  )
}
