// QRCodeOverlay - Placeholder component
export default function QRCodeOverlay({ show, url, onClose }) {
  if (!show) return null

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(0,0,0,0.8)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
        cursor: 'pointer'
      }}
      onClick={onClose}
    >
      <div style={{
        backgroundColor: 'white',
        padding: '2rem',
        borderRadius: '1rem',
        textAlign: 'center'
      }}>
        <p style={{ marginBottom: '1rem', fontSize: '1.2rem' }}>Scannez pour rejoindre</p>
        <p style={{ color: '#666', fontSize: '0.9rem' }}>{url}</p>
        <p style={{ marginTop: '1rem', color: '#999', fontSize: '0.8rem' }}>Cliquez pour fermer</p>
      </div>
    </div>
  )
}
