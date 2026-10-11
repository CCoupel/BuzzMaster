import { useEffect, useRef } from 'react'
import './RoundGroup.css'

/**
 * Groupe de l'éditeur de questions (#245, v11.2) : « Sans manche » (en premier) ou une manche. Maquette VALIDÉE
 * admin_questions_list__rounds_entractes_delete_choice (f″) : en-tête ⋮⋮ ▸/▾ nom compteur ▲▼ ✎ 🗑, corps = cartes,
 * « + Question » en pied de manche. Composant de présentation : l'état et les envois ROUNDS_UPDATE sont dans la page.
 */
export default function RoundGroup({
  unassigned = false,
  title,
  name,
  index = 0,
  total = 0,
  countText,
  typesText = '',
  collapsed = false,
  onToggle,
  editing = false,
  isNew = false,
  nameDraft = '',
  onNameChange,
  onNameValidate,
  onNameCancel,
  withEntractes = true,
  onWithEntractesChange,
  onUp,
  onDown,
  onRename,
  onDelete,
  onAddQuestion,
  isEmpty = false,
  emptyText,
  dragOver = false,
  headDrag,
  bodyDrag,
  children,
}) {
  const inputRef = useRef(null)
  useEffect(() => { if (editing) inputRef.current?.focus() }, [editing])
  const nameValid = nameDraft.trim().length > 0

  return (
    <section
      className={`round-group${unassigned ? ' unassigned' : ''}${collapsed ? ' collapsed' : ''}${dragOver ? ' dragover' : ''}`}
      aria-label={unassigned ? 'Sans manche' : (name || title)}
    >
      <div className="round-head" {...(headDrag?.drop || {})}>
        {unassigned ? (
          <span className="round-handle-spacer" aria-hidden="true" />
        ) : (
          <span
            className="round-handle"
            title="Glisser la manche"
            aria-hidden="true"
            draggable={!editing}
            {...(headDrag?.handle || {})}
          >⋮⋮</span>
        )}
        <button
          type="button"
          className="round-toggle"
          aria-expanded={!collapsed}
          aria-label={collapsed ? 'Déplier' : 'Replier'}
          onClick={onToggle}
        >{collapsed ? '▸' : '▾'}</button>
        <span className="round-name">
          {editing ? (
            <input
              ref={inputRef}
              className="round-name-input"
              value={nameDraft}
              maxLength={60}
              placeholder="Nom de la manche (60 caractères max)"
              aria-label="Nom de la manche"
              aria-invalid={!nameValid}
              onChange={(e) => onNameChange?.(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') { e.preventDefault(); if (nameValid) onNameValidate?.() }
                if (e.key === 'Escape') { e.preventDefault(); onNameCancel?.() }
              }}
            />
          ) : title}
        </span>
        {editing && isNew && (
          <label className="round-cb">
            <input
              type="checkbox"
              checked={withEntractes}
              onChange={(e) => onWithEntractesChange?.(e.target.checked)}
            /> Créer les entractes de début et de fin
          </label>
        )}
        <span className="round-count">{countText}</span>
        {collapsed && typesText && <span className="round-types">{typesText}</span>}
        {!unassigned && (
          <>
            <button type="button" className="round-ibtn" aria-label="Monter la manche" title="Monter" disabled={index === 0} onClick={onUp}>▲</button>
            <button type="button" className="round-ibtn" aria-label="Descendre la manche" title="Descendre" disabled={index === total - 1} onClick={onDown}>▼</button>
            {editing ? (
              <>
                <button type="button" className="round-ibtn" aria-label="Valider le nom" title="Valider" disabled={!nameValid} onClick={onNameValidate}>✓</button>
                <button type="button" className="round-ibtn" aria-label="Annuler" title="Annuler" onClick={onNameCancel}>✕</button>
              </>
            ) : (
              <button type="button" className="round-ibtn" aria-label="Renommer la manche" title="Renommer" onClick={onRename}>✎</button>
            )}
            {!isNew && (
              <button type="button" className="round-ibtn" aria-label="Supprimer la manche" title="Supprimer la manche" onClick={onDelete}>🗑</button>
            )}
          </>
        )}
      </div>
      {!collapsed && (
        <div className="round-body" {...(bodyDrag || {})}>
          {children}
          {isEmpty && emptyText ? <div className="round-empty">{emptyText}</div> : null}
          {!unassigned && !isNew && (
            <button type="button" className="round-add-q" aria-label={`Ajouter une question à ${name || title}`} onClick={onAddQuestion}>+ Question</button>
          )}
        </div>
      )}
    </section>
  )
}
