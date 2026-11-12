import { useState } from 'react'
import Button from './Button'
import { Modal } from './LightEffectsImport'

// #210 (v11.8.0, LOT-2, F2.3) — fenêtre « Nouvel effet » / « Dupliquer » : nom et identifiant (nom de fichier et référence dans les
// réglages, non modifiable ensuite). Maquette effects_catalogue_import_export §04 ; contrat §11.4 (identifiants), §4.8 (réservés).

// Identifiant proposé d'après un nom : minuscules sans accents, chiffres et tirets, 40 caractères au plus.
export const slugify = (name) => String(name).normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40).replace(/-+$/g, '')
const ID_RE = /^[a-z0-9-]{1,40}$/
// Identifiants réservés (contrat §11.4, §4.8) : mots de route et noms de périphériques Windows ; le serveur reste juge.
const ROUTE_WORD = /^(reload|try|draft|import)$/
const WINDOWS_DEVICE = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/
// Même texte que le serveur (§4.8, avec « identifiant » à la place de « clé id »).
const reservedMessage = (id) => (ROUTE_WORD.test(id) ? `« ${id} » est un mot réservé : changez l'identifiant`
  : WINDOWS_DEVICE.test(id) ? `« ${id} » est un nom réservé par Windows : changez l'identifiant` : '')

// Fenêtre « Nouvel effet » / « Dupliquer » : nom et identifiant (nom de fichier, non modifiable ensuite).
export default function NameIdDialog({ title, initialName, initialId, takenIds, confirmLabel, onConfirm, onCancel }) {
  const [name, setName] = useState(initialName)
  const [id, setId] = useState(initialId)
  const [idTouched, setIdTouched] = useState(!!initialId)
  const idError = !ID_RE.test(id) ? "Lettres minuscules, chiffres et tirets (40 au plus)." : takenIds.includes(id) ? 'Cet identifiant est déjà utilisé.' : reservedMessage(id)
  const ok = name.trim() !== '' && !idError
  return (
    <Modal titleId="lfx-name-title" title={title} onCancel={onCancel}>
      <div className="lfx-form">
        <label>Nom<input value={name} onChange={(e) => { setName(e.target.value); if (!idTouched) setId(slugify(e.target.value)) }} /></label>
        <label>Identifiant<input value={id} onChange={(e) => { setId(e.target.value); setIdTouched(true) }} aria-invalid={!!idError} /></label>
      </div>
      {idError && id !== '' && <p className="lfx-modal-err" role="alert">{idError}</p>}
      <p>L'identifiant sert de nom de fichier et de référence dans les réglages : lettres minuscules, chiffres et tirets. Il ne pourra plus être changé.</p>
      <div className="lfx-modal-actions">
        <Button variant="primary" size="sm" disabled={!ok} onClick={() => onConfirm({ id, name: name.trim() })}>{confirmLabel}</Button>
        <Button variant="secondary" size="sm" onClick={onCancel}>Annuler</Button>
      </div>
    </Modal>
  )
}

