import { useEffect, useRef } from 'react'
import QRCode from 'qrcode'

/**
 * QRCodeDisplay — renders a QR code canvas with optional color tint and center logo.
 * @param {string}  url      — Content to encode
 * @param {number}  size     — Canvas size in px (default 200)
 * @param {string}  label    — Optional text below canvas
 * @param {string}  fgColor  — QR foreground color (default '#000000')
 * @param {string}  logo     — Optional emoji/text overlaid at center (default null)
 */
export default function QRCodeDisplay({ url, size = 200, label, fgColor = '#000000', logo = null }) {
  const canvasRef = useRef(null)

  useEffect(() => {
    if (!canvasRef.current || !url) return

    QRCode.toCanvas(canvasRef.current, url, {
      width: size,
      margin: 2,
      errorCorrectionLevel: logo ? 'H' : 'M', // H (30%) required when a logo is overlaid
      color: {
        dark: fgColor,
        light: '#FFFFFF',
      },
    }, (error) => {
      if (error) console.error('QR Code generation error:', error)
    })
  }, [url, size, fgColor, logo])

  const logoSize = Math.round(size * 0.15) // ~15% of QR size (safe under H correction level)

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: '0.5rem',
    }}>
      <div style={{ position: 'relative', display: 'inline-block' }}>
        <canvas ref={canvasRef} style={{ borderRadius: '0.75rem', display: 'block' }} />
        {logo && (
          <div style={{
            position: 'absolute',
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            background: 'white',
            borderRadius: '8px',
            padding: '4px 5px',
            fontSize: `${logoSize}px`,
            lineHeight: 1,
            boxShadow: '0 0 0 3px white',
            pointerEvents: 'none',
            userSelect: 'none',
          }}>
            {logo}
          </div>
        )}
      </div>
      {label && <span style={{ fontSize: '0.9rem', color: '#666', fontWeight: 600 }}>{label}</span>}
    </div>
  )
}
