import { useEffect, useRef } from 'react'
import QRCode from 'qrcode'

export default function QRCodeDisplay({ url, size = 200, label }) {
  const canvasRef = useRef(null)

  useEffect(() => {
    if (!canvasRef.current || !url) return

    QRCode.toCanvas(canvasRef.current, url, {
      width: size,
      margin: 2,
      color: {
        dark: '#000000',
        light: '#FFFFFF',
      },
    }, (error) => {
      if (error) console.error('QR Code generation error:', error)
    })
  }, [url, size])

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: '0.5rem'
    }}>
      <canvas ref={canvasRef} />
      {label && <span style={{ fontSize: '0.9rem', color: '#666', fontWeight: 600 }}>{label}</span>}
    </div>
  )
}
