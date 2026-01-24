// QRCodeDisplay - Placeholder component
export default function QRCodeDisplay({ url, size = 200, label }) {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: '0.5rem'
    }}>
      <div style={{
        width: size,
        height: size,
        backgroundColor: '#f0f0f0',
        border: '2px solid #ddd',
        borderRadius: '0.5rem',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: '0.8rem',
        color: '#666',
        textAlign: 'center',
        padding: '1rem'
      }}>
        QR Code<br />{url}
      </div>
      {label && <span style={{ fontSize: '0.9rem', color: '#666' }}>{label}</span>}
    </div>
  )
}
