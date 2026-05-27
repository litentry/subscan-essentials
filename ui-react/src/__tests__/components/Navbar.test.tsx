import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Navbar } from '@/components/navbar'
import { useRouter } from 'next/router'
import '@testing-library/jest-dom'
import { DataProvider } from '@/context'
import * as api from '@/utils/api'

// Mock next/router
jest.mock('next/router', () => ({
  useRouter: jest.fn(),
}))

// Mock API hooks
jest.mock('@/utils/api', () => ({
  ...jest.requireActual('@/utils/api'),
  useMetadata: jest.fn(),
  useToken: jest.fn(),
  checkSearchHash: jest.fn(),
}))

describe('Navbar', () => {
  const mockRouter = {
    push: jest.fn(),
  }

  const mockMetadata = {
    networkNode: 'default',
    token: 'TST',
    ss58Format: 42,
    tokenDecimals: 12,
    enable_substrate: true,
    enable_evm: true,
  }

  const mockToken = { TST: { price: '100', change: '1' } }

  beforeEach(() => {
    ;(useRouter as jest.Mock).mockReturnValue(mockRouter)
    ;(api.useMetadata as jest.Mock).mockReturnValue({ data: { data: mockMetadata } })
    ;(api.useToken as jest.Mock).mockReturnValue({ data: { data: mockToken } })
    ;(api.checkSearchHash as jest.Mock).mockResolvedValue({ code: 0, data: { hash_type: 'block' } })
  })

  afterEach(() => {
    jest.clearAllMocks()
  })

  const renderNavbar = () => {
    return render(
      <DataProvider>
        <Navbar value="" />
      </DataProvider>
    )
  }

  it('renders the Heima brand', () => {
    renderNavbar()
    expect(screen.getByText('Heima Explorer')).toBeInTheDocument()
  })

  it('handles search with enter key', async () => {
    renderNavbar()
    const searchInput = screen.getByPlaceholderText('Search')

    fireEvent.change(searchInput, { target: { value: '123456' } })
    fireEvent.keyDown(searchInput, { key: 'Enter' })

    await waitFor(() => expect(mockRouter.push).toHaveBeenCalledWith('/sub/block/123456'))
  })

  it('auto-detects a substrate account without using the selected block type', async () => {
    renderNavbar()
    const searchInput = screen.getByPlaceholderText('Search')
    const address = '47BHMeKG1Q36gU6WP9ZGiqFhEPF5BhfyTVn9NSaemMd9e9uP'

    fireEvent.change(searchInput, { target: { value: address } })
    fireEvent.keyDown(searchInput, { key: 'Enter' })

    await waitFor(() => expect(mockRouter.push).toHaveBeenCalledWith(`/sub/account/${address}`))
    expect(api.checkSearchHash).not.toHaveBeenCalled()
  })
})
