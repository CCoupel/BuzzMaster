/**
 * Tests for NetworkWarningBanner — issue #96
 *
 * The component is purely presentational (no props, no hooks).
 * The hide/show logic is in GamePage — tested in GamePage.test.jsx.
 */
import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import NetworkWarningBanner from './NetworkWarningBanner'

describe('NetworkWarningBanner — issue #96', () => {
  it('renders its banner element', () => {
    const { container } = render(<NetworkWarningBanner />)
    expect(container.firstChild).not.toBeNull()
  })
})
