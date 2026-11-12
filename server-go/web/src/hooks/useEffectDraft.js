import { useCallback, useEffect, useRef, useState } from 'react'
import { sameDocument } from '../utils/effectDocument'

// #210 (v11.8.0, LOT-2) — brouillon d'un effet : une source (grille ou texte), validée par le serveur après chaque modification.
// Contrat : contracts/light-effects.md §11.2 (ouvrir), §11.3 (`draft` : validation, conversion, aperçus), §11.4 (enregistrer),
// §11.8 (deux onglets, une seule source). Le navigateur n'analyse JAMAIS de YAML et ne calcule aucune trame : erreurs,
// aperçus (`previews[].frames[].vars`) et texte régénéré viennent de la réponse du serveur ; réponses périmées ignorées.
//
// useEffectDraft({ id, create, initialDoc, groups, moment })
//   id          identifiant de l'effet (GET …/source, PUT …/{id})
//   initialDoc  document de départ d'un NOUVEL effet ou d'une copie (pas de GET) ; avec `create`, le premier PUT crée le fichier
//   groups      tailles de salle à prévisualiser (§11.3 : 0 à 8 valeurs distinctes de 1 à 12) ; défaut 1, 3, 6, 12
//   moment      moment prévisualisé (facultatif : défaut du serveur)

export const DRAFT_DELAY_MS = 300
export const DEFAULT_PREVIEW_GROUPS = Object.freeze([1, 3, 6, 12])

const post = (url, body) => fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })

// Corps d'une réponse : JSON quand c'en est un, sinon le texte brut (les refus 400 / 413 / 503 du serveur sont du texte, `http.Error`).
async function readBody(res) {
  if (typeof res.text !== 'function') {
    try { return { data: await res.json(), raw: '' } } catch { return { data: {}, raw: '' } }
  }
  const raw = await res.text().catch(() => '')
  let data = {}
  try { data = raw ? JSON.parse(raw) : {} } catch { /* texte brut */ }
  return { data, raw: raw.trim() }
}

const networkDraft = (e) => ({ ok: false, errors: [{ line: 0, message: `Vérification impossible : ${e.message}` }], previews: [], network: true })

export function useEffectDraft({ id, create = false, initialDoc = null, groups = DEFAULT_PREVIEW_GROUPS, moment = '' } = {}) {
  const [status, setStatus] = useState('loading') // loading | ready | error
  const [loadError, setLoadError] = useState('')
  const [source, setSource] = useState(null) // réponse de GET …/source (ou synthétique pour un effet neuf)
  const [doc, setDocState] = useState(null) // document de la grille
  const [text, setTextState] = useState('')
  const [tab, setTab] = useState('grid')
  const [draft, setDraft] = useState(null) // dernière réponse de `draft` : { ok, errors, previews, yaml, grid, …, from }
  const [pending, setPending] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [persisted, setPersisted] = useState(false) // le fichier existe sur le serveur (télécharger possible)
  const [dirtyFlags, setDirtyFlags] = useState({ grid: false, text: false })

  const timer = useRef(null)
  const seq = useRef(0)
  const mounted = useRef(true)
  const last = useRef('grid') // dernière source modifiée : 'grid' | 'text'
  const gridModified = useRef(false) // la grille a été modifiée depuis l'ouverture (sinon le texte reste celui du fichier)
  const created = useRef(false)
  const initial = useRef({ doc: null, text: '' })
  const live = useRef({ doc: null, text: '' })
  const groupsKey = JSON.stringify(groups)
  const opts = useRef({ groups, moment })
  opts.current = { groups, moment }

  const runDraft = useCallback(async (body, from) => {
    const mine = ++seq.current
    setPending(true)
    const payload = { ...body, groups: opts.current.groups }
    if (opts.current.moment) payload.moment = opts.current.moment
    let out
    try {
      const res = await post('/api/lighting/effects/draft', payload)
      const { data, raw } = await readBody(res)
      out = res.ok ? data : { ok: false, errors: [{ line: 0, message: data?.error || raw || `Vérification impossible (HTTP ${res.status}).` }], previews: [] }
    } catch (e) {
      out = networkDraft(e)
    }
    if (!mounted.current || mine !== seq.current) return // réponse périmée : une modification plus récente est en route
    setDraft({ ...out, errors: out.errors || [], previews: out.previews || [], from })
    setPending(false)
    // Grille → Texte : le texte devient le fichier régénéré (commentaires perdus, §11.8) dès que la grille a été modifiée.
    if (from === 'grid' && out.ok && typeof out.yaml === 'string' && last.current === 'grid' && gridModified.current) {
      live.current.text = out.yaml
      setTextState(out.yaml)
    }
  }, [])

  const schedule = useCallback((body, from, immediate = false) => {
    clearTimeout(timer.current)
    setPending(true)
    seq.current += 1 // invalide une réponse en vol
    if (immediate) runDraft(body, from)
    else timer.current = setTimeout(() => runDraft(body, from), DRAFT_DELAY_MS)
  }, [runDraft])

  const currentBody = useCallback(() => (last.current === 'text' ? { yaml: live.current.text } : { effect: live.current.doc }), [])

  // ---- ouverture ----
  useEffect(() => {
    mounted.current = true
    let cancelled = false
    async function open() {
      setStatus('loading'); setLoadError(''); setDraft(null)
      if (initialDoc) {
        const src = { id: initialDoc.id, origin: 'user', read_only: false, yaml: '', effect: initialDoc, has_comments: false, custom_order: false, grid: { editable: true, effect: initialDoc, converted: [], blocked: [] } }
        setSource(src); setDocState(initialDoc); setTextState('')
        initial.current = { doc: initialDoc, text: '' }; live.current = { doc: initialDoc, text: '' }
        last.current = 'grid'; gridModified.current = false; created.current = false; setPersisted(!create)
        setStatus('ready')
        schedule({ effect: initialDoc }, 'grid', true)
        return
      }
      try {
        const res = await fetch(`/api/lighting/effects/${encodeURIComponent(id)}/source`)
        if (!res.ok) throw new Error(res.status === 404 ? 'Effet inconnu — rechargez le catalogue.' : `Ouverture impossible (HTTP ${res.status}).`)
        const data = await res.json()
        if (cancelled) return
        const editable = data.grid?.editable !== false && !!data.grid?.effect
        const start = editable ? data.grid.effect : data.effect
        setSource(data); setDocState(start); setTextState(data.yaml || '')
        initial.current = { doc: start, text: data.yaml || '' }; live.current = { doc: start, text: data.yaml || '' }
        last.current = editable ? 'grid' : 'text'; gridModified.current = false; created.current = true; setPersisted(true)
        setTab('grid')
        setStatus('ready')
        // Aperçus et état de validation initial : brouillon de la source ouverte (rien n'est modifié).
        schedule(editable ? { effect: start } : { yaml: data.yaml || '' }, editable ? 'grid' : 'text', true)
      } catch (e) {
        if (!cancelled) { setLoadError(e.message); setStatus('error') }
      }
    }
    open()
    return () => { cancelled = true; mounted.current = false; clearTimeout(timer.current); seq.current += 1 }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, initialDoc])

  // Tailles d'aperçu / moment modifiés (zones réelles) : même brouillon, nouveaux aperçus.
  useEffect(() => {
    if (status !== 'ready') return
    schedule(currentBody(), last.current, false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groupsKey, moment])

  const readOnly = !!source?.read_only
  const gridEditable = source ? source.grid?.editable !== false && !!source.grid?.effect : true

  // ---- modifications ----
  const setDoc = useCallback((next) => {
    if (readOnly) return
    const value = typeof next === 'function' ? next(live.current.doc) : next
    live.current.doc = value
    last.current = 'grid'
    gridModified.current = true
    setDocState(value)
    setDirtyFlags(f => ({ ...f, grid: true }))
    setSaveError('')
    schedule({ effect: value }, 'grid')
  }, [readOnly, schedule])

  const setText = useCallback((value) => {
    if (readOnly) return
    live.current.text = value
    last.current = 'text'
    setTextState(value)
    setDirtyFlags(f => ({ ...f, text: true }))
    setSaveError('')
    schedule({ yaml: value }, 'text')
  }, [readOnly, schedule])

  // Changement d'onglet. Texte → Grille : seulement si le texte est valide ET convertible en grille (§11.8).
  const switchTab = useCallback((to) => {
    if (to === tab) return { ok: true }
    if (to === 'grid' && last.current === 'text' && dirtyFlags.text) {
      if (pending || !draft || draft.from !== 'text') return { ok: false, reason: 'pending' }
      if (!draft.ok) return { ok: false, reason: 'invalid' }
      if (!draft.grid || draft.grid.editable === false || !draft.grid.effect) return { ok: false, reason: 'not-editable' }
      live.current.doc = draft.grid.effect
      setDocState(draft.grid.effect)
    }
    setTab(to)
    return { ok: true }
  }, [tab, dirtyFlags.text, pending, draft])

  const dirty = (dirtyFlags.grid && !sameDocument(doc, initial.current.doc)) || (dirtyFlags.text && text !== initial.current.text)

  const canSave = !readOnly && !pending && !saving && !!draft?.ok && (dirty || (!!initialDoc && create && !created.current))

  // ---- enregistrement (§11.4) ----
  const save = useCallback(async () => {
    if (!canSave) return { ok: false, message: 'Rien à enregistrer.' }
    setSaving(true); setSaveError('')
    try {
      const useText = last.current === 'text' && live.current.text !== ''
      const body = useText ? { yaml: live.current.text } : { effect: live.current.doc }
      if (create && !created.current) body.create = true
      const res = await fetch(`/api/lighting/effects/${encodeURIComponent(id)}`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
      })
      const { data, raw } = await readBody(res)
      if (res.ok) {
        created.current = true
        setPersisted(true)
        initial.current = { doc: live.current.doc, text: live.current.text }
        setDirtyFlags({ grid: false, text: false })
        return { ok: true, catalogue: data?.catalogue }
      }
      let message
      if (res.status === 409) {
        message = data?.reason === 'exists' ? 'Un effet porte déjà cet identifiant.'
          : data?.reason === 'limit' ? 'Le dossier contient déjà 100 effets : supprimez-en avant d\'enregistrer.'
          : 'Un effet livré ne peut pas être modifié : dupliquez-le.'
      } else if (res.status === 404) message = 'Effet inconnu — rechargez le catalogue.'
      else if (res.status === 413) message = 'Fichier trop gros (64 Ko au plus).'
      else if (res.status === 503) message = 'Stockage des effets indisponible.'
      else if (res.status === 400 && !data?.errors?.length && raw) message = `Enregistrement refusé : ${raw}.`
      else if (res.status === 400 && data?.errors?.length) {
        message = 'Le serveur refuse cet effet.'
        setDraft(d => ({ ...(d || {}), ok: false, errors: data.errors, from: last.current }))
      } else message = `Enregistrement impossible (HTTP ${res.status}).`
      setSaveError(message)
      return { ok: false, message, errors: data?.errors }
    } catch (e) {
      const message = `Enregistrement impossible : ${e.message}`
      setSaveError(message)
      return { ok: false, message }
    } finally { setSaving(false) }
  }, [canSave, create, id])

  return {
    status, loadError, source, readOnly, gridEditable,
    doc, text, tab, setTab: switchTab, setDoc, setText,
    draft, pending, errors: draft?.errors || [], previews: draft?.previews || [],
    dirty, canSave, saving, saveError, save, persisted,
    // Avertissements d'ouverture (§11.2, §11.8)
    hasComments: !!source?.has_comments, customOrder: !!source?.custom_order,
    converted: source?.grid?.converted || [], blocked: source?.grid?.blocked || [],
    lastSource: last.current,
  }
}
