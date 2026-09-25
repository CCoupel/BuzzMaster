import './BrandLogo.css'

/** Mot-symbole BuzzControl (logo A1, #239). Décoratif : le bouton porte l'aria-label. */
export default function BrandLogo() {
  return (
    <span className="brand-logo-wordmark" aria-hidden="true">
      <span className="brand-logo-buzz">Buzz</span>
      <span className="brand-logo-control">Control</span>
      <span className="brand-logo-bolt">⚡</span>
    </span>
  )
}
