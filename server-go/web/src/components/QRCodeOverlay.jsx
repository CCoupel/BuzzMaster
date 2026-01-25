import { motion, AnimatePresence } from 'framer-motion'
import QRCodeDisplay from './QRCodeDisplay'
import { useGame } from '../hooks/GameContext'

export default function QRCodeOverlay({ show }) {
  const { gameState } = useGame()

  // Build enrollment URL
  const enrollUrl = `http://${window.location.hostname}/`

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            backgroundColor: 'rgba(0,0,0,0.85)',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            gap: '2rem',
            padding: '2rem',
          }}
        >
          <motion.div
            initial={{ scale: 0.8, y: -20 }}
            animate={{ scale: 1, y: 0 }}
            exit={{ scale: 0.8, y: -20 }}
            transition={{ delay: 0.1 }}
            style={{
              backgroundColor: 'white',
              padding: '3rem',
              borderRadius: '24px',
              textAlign: 'center',
              boxShadow: '0 20px 60px rgba(0, 0, 0, 0.5)',
            }}
          >
            <h2 style={{ margin: '0 0 1.5rem 0', fontSize: '2rem', color: '#333' }}>
              Scannez pour rejoindre
            </h2>
            <QRCodeDisplay url={enrollUrl} size={300} />
          </motion.div>

          {/* Progress bar */}
          <motion.div
            initial={{ scale: 0.8 }}
            animate={{ scale: 1 }}
            exit={{ scale: 0.8 }}
            transition={{ delay: 0.2 }}
            style={{
              backgroundColor: 'rgba(255, 255, 255, 0.1)',
              borderRadius: '16px',
              padding: '1.5rem 2rem',
              minWidth: '300px',
              textAlign: 'center',
            }}
          >
            <p style={{ color: 'white', fontSize: '1.2rem', margin: '0 0 0.5rem 0', fontWeight: 600 }}>
              {gameState.virtualPlayerCount || 0} / {gameState.virtualPlayerLimit || 20} joueurs
            </p>
            <div style={{
              height: '8px',
              backgroundColor: 'rgba(255, 255, 255, 0.2)',
              borderRadius: '4px',
              overflow: 'hidden',
            }}>
              <motion.div
                initial={{ width: 0 }}
                animate={{
                  width: `${((gameState.virtualPlayerCount || 0) / (gameState.virtualPlayerLimit || 20)) * 100}%`
                }}
                transition={{ duration: 0.5 }}
                style={{
                  height: '100%',
                  backgroundColor: '#22c55e',
                  borderRadius: '4px',
                }}
              />
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
