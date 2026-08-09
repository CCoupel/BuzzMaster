import { useEffect } from 'react'
import './ApiKeyHelpModal.css'

// Contenu d'aide par fournisseur IA (bugfix/config-api-key-help).
// Statique : pas d'appel réseau, juste des liens directs vers les consoles.
const PROVIDER_INFO = {
  anthropic: {
    name: 'Claude (Anthropic)',
    badge: 'Payant',
    badgeClass: 'paid',
    signupUrl: 'https://console.anthropic.com',
    signupLabel: 'console.anthropic.com',
    keysUrl: 'https://console.anthropic.com/settings/keys',
    keysLabel: 'console.anthropic.com/settings/keys',
    note: "Nécessite l'ajout d'un moyen de paiement (facturation à l'usage) avant de pouvoir générer une clé.",
  },
  groq: {
    name: 'Groq',
    badge: 'Gratuit — recommandé',
    badgeClass: 'free',
    signupUrl: 'https://console.groq.com',
    signupLabel: 'console.groq.com',
    keysUrl: 'https://console.groq.com/keys',
    keysLabel: 'console.groq.com/keys',
    note: 'Aucune carte bancaire requise pour le tier gratuit.',
  },
}

export default function ApiKeyHelpModal({ provider, onClose }) {
  const info = PROVIDER_INFO[provider]

  // Fermeture via Echap, cohérent avec les autres modales (USBConfigModal)
  useEffect(() => {
    const handleKey = (e) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  if (!info) return null

  return (
    <div className="apikey-help-overlay" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className="apikey-help-modal" role="dialog" aria-modal="true" aria-labelledby="apikey-help-title">
        <div className="apikey-help-header">
          <h2 id="apikey-help-title">Obtenir une clé API {info.name}</h2>
          <button className="apikey-help-close" onClick={onClose} aria-label="Fermer">&times;</button>
        </div>

        <div className="apikey-help-body">
          <span className={`apikey-help-badge ${info.badgeClass}`}>{info.badge}</span>

          <ol className="apikey-help-steps">
            <li>
              <strong>Créer un compte</strong>
              <p>Inscrivez-vous sur la console {info.name}.</p>
              <a className="btn btn-secondary btn-sm" href={info.signupUrl} target="_blank" rel="noopener noreferrer">
                Ouvrir {info.signupLabel} ↗
              </a>
            </li>
            <li>
              <strong>Générer une clé API</strong>
              <p>Une fois connecté(e), rendez-vous dans la section des clés API et créez-en une nouvelle.</p>
              {info.note && <p className="apikey-help-note">{info.note}</p>}
              <a className="btn btn-primary btn-sm" href={info.keysUrl} target="_blank" rel="noopener noreferrer">
                Ouvrir {info.keysLabel} ↗
              </a>
            </li>
            <li>
              <strong>Coller la clé dans BuzzControl</strong>
              <p>
                Copiez la clé générée (elle ne sera plus jamais affichée par le fournisseur), collez-la dans le
                champ « Clé API {info.name} », puis cliquez sur « Enregistrer ».
              </p>
            </li>
          </ol>
        </div>
      </div>
    </div>
  )
}
