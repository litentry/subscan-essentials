import React, { useState, KeyboardEvent, useMemo, useEffect } from 'react'
import { useRouter } from 'next/router'

import { BareProps } from '@/types/page'
import {
  Navbar,
  NavbarBrand,
  NavbarContent,
  NavbarItem,
  Button,
  DropdownItem,
  DropdownMenu,
  Dropdown,
  DropdownTrigger,
  Link,
  Input,
  Select,
  SelectItem,
  Divider,
} from '@heroui/react'
import { useData } from '@/context'
import Image from 'next/image'
import _ from 'lodash'
import { env } from 'next-runtime-env'
import { checkSearchHash } from '@/utils/api'

interface Props extends BareProps {
  value: string
}

type SearchType = 'auto' | 'sub_block' | 'sub_extrinsic' | 'sub_event' | 'sub_account' | 'pvm_block' | 'pvm_tx' | 'pvm_contract' | 'pvm_account'

const evmAddressRegex = /^(0x)?[0-9a-fA-F]{40}$/
const substrateAccountRegex = /^[1-9A-HJ-NP-Za-km-z]{32,60}$/
const ChevronDown = ({ fill, size, ...props }: { fill?: string; size?: number | string } & React.SVGProps<SVGSVGElement>) => {
  return (
    <svg fill="none" height={size || 24} viewBox="0 0 24 24" width={size || 24} xmlns="http://www.w3.org/2000/svg" {...props}>
      <path
        d="m19.92 8.95-6.52 6.52c-.77.77-2.03.77-2.8 0L4.08 8.95"
        stroke={fill}
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeMiterlimit={10}
        strokeWidth={1.5}
      />
    </svg>
  )
}

const SearchIcon = ({ size = 24, strokeWidth = 1.5, ...props }) => {
  return (
    <svg aria-hidden="true" fill="none" focusable="false" height={size} role="presentation" viewBox="0 0 24 24" width={size} {...props}>
      <path
        d="M11.5 21C16.7467 21 21 16.7467 21 11.5C21 6.25329 16.7467 2 11.5 2C6.25329 2 2 6.25329 2 11.5C2 16.7467 6.25329 21 11.5 21Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={strokeWidth}
      />
      <path d="M22 22L20 20" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth={strokeWidth} />
    </svg>
  )
}

const Component: React.FC<Props> = ({ children, className }) => {
  const { metadata, token } = useData()
  const [value, setValue] = useState('')
  const [type, setType] = useState<string[]>(['auto'])
  const router = useRouter()

  const showSubstrate = metadata?.enable_substrate
  const showPVM = metadata?.enable_evm
  const networkNode = metadata?.networkNode || 'heima'
  const explorerName = env('NEXT_PUBLIC_EXPLORER_NAME') || 'Heima Explorer'
  const networkLogoSrc = networkNode === 'heima' ? '/images/network/heima/logo.svg' : `/images/network/${networkNode}/logo.png`

  const icons = {
    chevron: <ChevronDown fill="currentColor" size={16} />,
    search: <SearchIcon fill="none" size={16} />,
  }
  const typeOptions = useMemo(() => {
    const autoOptions = [
      {
        name: 'Auto Detect',
        value: 'auto',
      },
    ]
    const subOptions = [
      {
        name: 'Substrate Block',
        value: 'sub_block',
      },
      {
        name: 'Substrate Extrinsic',
        value: 'sub_extrinsic',
      },
      {
        name: 'Substrate Event',
        value: 'sub_event',
      },
      {
        name: 'Substrate Account',
        value: 'sub_account',
      },
    ]
    const pvmOptions = [
      {
        name: 'EVM Contract Block',
        value: 'pvm_block',
      },
      {
        name: 'EVM Contract Transaction',
        value: 'pvm_tx',
      },
      {
        name: 'EVM Contract',
        value: 'pvm_contract',
      },
      {
        name: 'EVM Account',
        value: 'pvm_account',
      },
    ]
    let options: any[] = [...autoOptions]
    if (metadata?.enable_substrate) {
      _.forEach(subOptions, (item) => {
        options.push({
          name: item.name,
          value: item.value,
        })
      })
    }
    if (metadata?.enable_evm) {
      _.forEach(pvmOptions, (item) => {
        options.push({
          name: item.name,
          value: item.value,
        })
      })
    }
    return options
  }, [metadata?.enable_substrate, metadata?.enable_evm])

  const handleSearch = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      void handleRedirect()
    }
  }

  const routeForSearchType = (searchType: SearchType | '', search: string) => {
    switch (searchType) {
      case 'sub_block':
        return `/sub/block/${search}`
      case 'sub_extrinsic':
        return `/sub/extrinsic/${search}`
      case 'sub_event':
        return `/sub/event/${search}`
      case 'sub_account':
        return `/sub/account/${search}`
      case 'pvm_block':
        return `/block/${search}`
      case 'pvm_tx':
        return `/tx/${search}`
      case 'pvm_contract':
        return `/contract/${search}`
      case 'pvm_account':
        return `/address/${search}`
      default:
        return ''
    }
  }

  const routeTypeFromHashType = (hashType: string): SearchType | '' => {
    switch (hashType) {
      case 'block':
        return 'sub_block'
      case 'extrinsic':
        return 'sub_extrinsic'
      case 'address':
        return 'sub_account'
      case 'evm_block':
        return 'pvm_block'
      case 'evm_transaction':
        return 'pvm_tx'
      case 'evm_contract':
        return 'pvm_contract'
      case 'evm_address':
        return 'pvm_account'
      default:
        return ''
    }
  }

  const detectSearchType = async (search: string): Promise<SearchType | ''> => {
    if (showSubstrate !== false && substrateAccountRegex.test(search) && !evmAddressRegex.test(search)) {
      return 'sub_account'
    }

    try {
      const detected = await checkSearchHash(env('NEXT_PUBLIC_API_HOST') || '', { hash: search })
      if (detected?.code === 0 && detected.data?.hash_type) {
        return routeTypeFromHashType(detected.data.hash_type)
      }
    } catch (error) {
      // Keep the manual search selector usable when the backend does not support auto-detection yet.
    }

    if (showPVM !== false && evmAddressRegex.test(search)) {
      return 'pvm_account'
    }
    return showSubstrate !== false ? 'sub_block' : showPVM ? 'pvm_block' : ''
  }

  const handleRedirect = async () => {
    const search = value.trim()
    if (!search) {
      return
    }

    const selectedType = (type[0] || 'auto') as SearchType
    const searchType = selectedType === 'auto' ? await detectSearchType(search) : selectedType
    const route = routeForSearchType(searchType, search)
    if (route) {
      router.push(route)
      setValue('')
    }
  }
  useEffect(() => {
    if (metadata?.enable_evm && !metadata?.enable_substrate) {
      setType(['auto'])
    }
  }, [metadata?.enable_evm, metadata?.enable_substrate])

  const bannerBackgroundImage = useMemo(() => {
    return `
      radial-gradient(circle at 12% 8%, rgba(156, 249, 130, 0.28), transparent 18rem),
      radial-gradient(circle at 82% 0%, rgba(18, 198, 197, 0.24), transparent 22rem),
      linear-gradient(180deg, #131517 0%, #101214 55%, #000000 100%)
    `
  }, [])

  return (
    <div
      className="bg-no-repeat bg-cover bg-center"
      style={{
        backgroundImage: bannerBackgroundImage,
        backgroundSize: 'cover',
      }}>
      <Navbar
        position="static"
        maxWidth="full"
        classNames={{
          wrapper: 'px-0 w-full lg:max-w-screen-xl',
          base: 'bg-transparent backdrop-filter-none',
        }}>
        <NavbarBrand>
          <Link href="/" className="text-inherit">
            <div className="flex items-center gap-2 text-white">
              <span className="text-xl font-semibold tracking-normal">{explorerName}</span>
              <span className="hidden rounded-md border border-white/15 bg-white/10 px-2 py-0.5 text-xs font-medium text-[#AEFEC3] sm:inline">Powered by Subscan Essentials</span>
            </div>
          </Link>
        </NavbarBrand>
        <NavbarContent className="hidden sm:flex gap-4" justify="end">
          {showSubstrate && showPVM && (
            <>
              <Dropdown
                classNames={{
                  content: 'border border-white/10 !bg-[#121415] text-white shadow-[0_18px_48px_rgba(0,0,0,0.38)]',
                }}>
                <NavbarItem>
                  <DropdownTrigger>
                    <Button
                      disableRipple
                      size="lg"
                      className="p-0 bg-transparent data-[hover=true]:bg-transparent text-white"
                      radius="sm"
                      variant="light"
                      endContent={icons.chevron}>
                      Substrate
                    </Button>
                  </DropdownTrigger>
                </NavbarItem>
                <DropdownMenu
                  aria-label="Substrate"
                  itemClasses={{
                    base: 'gap-4 text-white data-[hover=true]:bg-white/10 data-[selectable=true]:focus:bg-white/10',
                    title: 'text-white',
                  }}>
                  <DropdownItem key="block">
                    <Link href="/sub/block" className="block text-inherit">
                      Block
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="extrinsic">
                    <Link href="/sub/extrinsic" className="block text-inherit">
                      Extrinsic
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="account">
                    <Link href="/sub/account" className="block text-inherit">
                      Account
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="event">
                    <Link href="/sub/event" className="block text-inherit">
                      Event
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="transfer">
                    <Link href="/sub/transfer" className="block text-inherit">
                      Transfer
                    </Link>
                  </DropdownItem>
                </DropdownMenu>
              </Dropdown>
              <Dropdown
                classNames={{
                  content: 'border border-white/10 !bg-[#121415] text-white shadow-[0_18px_48px_rgba(0,0,0,0.38)]',
                }}>
                <NavbarItem>
                  <DropdownTrigger>
                    <Button
                      disableRipple
                      size="lg"
                      className="p-0 bg-transparent data-[hover=true]:bg-transparent text-white"
                      radius="sm"
                      variant="light"
                      endContent={icons.chevron}>
                      EVM
                    </Button>
                  </DropdownTrigger>
                </NavbarItem>
                <DropdownMenu
                  aria-label="Substrate"
                  itemClasses={{
                    base: 'gap-4 text-white data-[hover=true]:bg-white/10 data-[selectable=true]:focus:bg-white/10',
                    title: 'text-white',
                  }}>
                  <DropdownItem key="block">
                    <Link href="/block" className="block text-inherit">
                      Block
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="extrinsic">
                    <Link href="/tx" className="block text-inherit">
                      Transaction
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="account">
                    <Link href="/address" className="block text-inherit">
                      Account
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="contract">
                    <Link href="/contract" className="block text-inherit">
                      Contract
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="erc20">
                    <Link href="/erc20_token" className="block text-inherit">
                      ERC-20
                    </Link>
                  </DropdownItem>
                  <DropdownItem key="erc721">
                    <Link href="/erc721_token" className="block text-inherit">
                      ERC-721
                    </Link>
                  </DropdownItem>
                </DropdownMenu>
              </Dropdown>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/sub/extrinsic">
                  Extrinsic
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/tx">
                  Transaction
                </Link>
              </NavbarItem>
            </>
          )}
          {showSubstrate && !showPVM && (
            <>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/sub/block">
                  Block
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/sub/extrinsic">
                  Extrinsic
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/sub/account">
                  Account
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/sub/event">
                  Event
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/sub/transfer">
                  Transfer
                </Link>
              </NavbarItem>
            </>
          )}
          {!showSubstrate && showPVM && (
            <>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/block">
                  Block
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/tx">
                  Transaction
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/address">
                  Account
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/contract">
                  Contract
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/erc20_token">
                  ERC-20
                </Link>
              </NavbarItem>
              <NavbarItem>
                <Link className="text-white font-semibold" href="/erc721_token">
                  ERC-721
                </Link>
              </NavbarItem>
            </>
          )}
          <NavbarItem>
            <div className="rounded-lg border border-white/10 bg-white/10 px-2.5 py-0.5">
              <Image
                width={0}
                height={0}
                sizes="100vw"
                className="hidden md:block h-[30px] w-auto"
                src={networkLogoSrc}
                alt={networkNode}
              />
            </div>
          </NavbarItem>
        </NavbarContent>
      </Navbar>
      <div className="w-full px-4 pb-6 flex justify-center">
        <div className="w-full lg:max-w-screen-xl">
          <Input
            classNames={{
              base: 'lg:max-w-2xl max-w-full h-10 mx-auto',
              mainWrapper: 'h-full',
              input: 'text-small',
              inputWrapper: 'h-full font-normal bg-white p-0',
            }}
            value={value}
            onValueChange={setValue}
            onKeyDown={handleSearch}
            placeholder="Search"
            size="md"
            startContent={
              <div className="flex h-5 items-center">
                <Select
                  className=""
                  classNames={{
                    base: 'w-auto',
                    trigger: '!bg-transparent shadow-none !w-auto px-5',
                    innerWrapper: '!w-auto',
                    value: 'text-[#171819]',
                    listbox: 'text-white bg-transparent',
                    listboxWrapper: 'max-h-none overflow-visible',
                    selectorIcon: 'right-[0px]',
                  }}
                  popoverProps={{
                    classNames: {
                      content:
                        'heima-select-popover min-w-[280px] w-max max-h-none border border-white/10 !bg-[#121415] !text-white shadow-[0_18px_48px_rgba(0,0,0,0.38)]',
                    },
                  }}
                  listboxProps={{
                    className: 'max-h-none overflow-visible',
                    itemClasses: {
                      base: 'min-w-[260px] !text-white data-[hover=true]:!text-white data-[hover=true]:bg-white/10 data-[selectable=true]:focus:!text-white data-[selectable=true]:focus:bg-white/10 data-[selected=true]:!text-white data-[selected=true]:bg-white/10',
                      title: 'text-white whitespace-nowrap overflow-visible text-clip',
                      selectedIcon: 'text-[#9CF982]',
                    },
                  }}
                  label=""
                  aria-label="Search type"
                  selectedKeys={type}
                  onSelectionChange={(key) => {
                    if (key.currentKey) {
                      setType([key.currentKey])
                    }
                  }}>
                  {typeOptions.map((item) => (
                    <SelectItem key={item.value}>{item.name}</SelectItem>
                  ))}
                </Select>
                <Divider orientation="vertical" className="mx-4" />
              </div>
            }
            endContent={<SearchIcon fill="none" size={24} onClick={() => void handleRedirect()} className="mr-3 cursor-pointer" />}
          />
        </div>
      </div>
    </div>
  )
}

export default Component
