import { render, screen } from '@testing-library/react'
import '@testing-library/jest-dom'
import { Footer } from '@/components/Footer'

// Mock next/image because it's not available in the test environment
jest.mock('next/image', () => ({
  __esModule: true,
  default: (props: any) => {
    // eslint-disable-next-line jsx-a11y/alt-text
    return <img {...props} />
  }
}))

describe('Footer', () => {
  
  it('renders logo images', () => {
    render(<Footer />)
    
    expect(screen.getByAltText('polakdot')).toBeInTheDocument()
    expect(screen.getByAltText('subscan')).toBeInTheDocument()
  })
  
  it('renders attribution and source links', () => {
    render(<Footer />)
    
    const links = screen.getAllByRole('link')
    const hrefs = links.map(link => link.getAttribute('href'))
    
    expect(screen.getByText('Powered by')).toBeInTheDocument()
    expect(screen.getByText('Subscan Essentials')).toBeInTheDocument()
    expect(screen.getByText(/not an official Subscan-hosted service/i)).toBeInTheDocument()
    expect(hrefs).toContain('https://github.com/litentry/subscan-essentials-ui-react')
    expect(hrefs).toContain('https://github.com/litentry/subscan-essentials')
    expect(hrefs).toContain('https://www.gnu.org/licenses/gpl-3.0.en.html')
    
    // Check for X link
    expect(hrefs).toContain('https://x.com/heimaNetwork')
    
    // Check for Email link
    expect(hrefs).toContain('mailto:support@heima.network')
  })
})
