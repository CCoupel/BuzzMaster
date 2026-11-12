// #210 (v11.8.0, LOT-2) — onglet « Texte » : le fichier YAML tel qu'il sera écrit (maquette effect_editor §03, contrat §11.8).
// Le texte est vérifié par le serveur pendant la saisie (`draft` avec `yaml`) : les erreurs donnent la ligne et la cause. Le navigateur
// n'analyse rien. Tant que le texte contient une erreur, on ne peut ni enregistrer, ni essayer, ni revenir à la grille.
//
// Props : text, onChange(text), readOnly, errors[{line, message}], pending, hasComments (le fichier ouvert contient des « # »)

export default function EffectTextTab({ text, onChange, readOnly = false, errors = [], pending = false, hasComments = false }) {
  return (
    <div className="efx-text" data-testid="effect-text">
      <div className="efx-two">
        <div>
          <textarea
            className="efx-yaml" spellCheck={false} aria-label="Fichier de l'effet (YAML)" value={text}
            readOnly={readOnly} onChange={(e) => onChange(e.target.value)}
          />
        </div>
        <div>
          {errors.length > 0 && (
            <div className="efx-err" role="alert">
              {errors.map((er, i) => (
                <div key={i}>{er.line ? <b>Ligne {er.line}</b> : null}{er.line ? ' : ' : ''}{er.message}</div>
              ))}
            </div>
          )}
          {errors.length === 0 && pending && <p className="efx-hint" role="status">Vérification du texte…</p>}
          {errors.length === 0 && !pending && !readOnly && <p className="efx-ok" role="status">Le texte est valide.</p>}
          <p className="efx-hint">
            Le texte est celui du fichier ; tout le format y reste permis, y compris les voisins (<code>gA-1</code>, <code>gA+1</code>).
            Il est vérifié par le serveur pendant la saisie ; les erreurs donnent la ligne et la cause. Tant que le texte contient une erreur,
            on ne peut ni enregistrer, ni essayer, ni revenir à la grille.
          </p>
          {hasComments && !readOnly && (
            <div className="efx-warn">
              <b>Ce fichier contient des commentaires</b> (lignes qui commencent par <code>#</code>). Ils sont conservés tant que vous enregistrez
              depuis l'onglet Texte. Une modification faite dans la grille réécrit le fichier : les commentaires sont alors perdus.
            </div>
          )}
          <p className="efx-hint">Passer de Texte à Grille reprend le texte dans la grille. Passer de Grille à Texte montre le fichier tel qu'il sera enregistré.</p>
        </div>
      </div>
    </div>
  )
}
