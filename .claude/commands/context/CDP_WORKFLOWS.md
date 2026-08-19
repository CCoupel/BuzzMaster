# CDP_WORKFLOWS.md — Adaptations projet BuzzControl

> Compagnon de `CDP_WORKFLOWS.template.md` (déployé par sync template) — ne contient que les
> écarts assumés avec le template pour ce projet. Lu en plus du template par toute commande CDP
> (`/feature`, `/bugfix`, `/hotfix`, `/refactor`).

## Écart — Parallélisation Review / QA

**Décision projet (2026-08-19)**, en complément de la section 5 "Phase Review" / "Phase QA" du
template (qui décrit un enchaînement strictement séquentiel Review → QA).

### Principe

Quand le risque de REJECTED en Review est jugé faible, `qa` peut être dispatché **en parallèle**
de `code-reviewer`, dès que `test-writer` a terminé (qa dépend des scripts de test commités par
test-writer — jamais dispatché avant son DONE). Gain net garanti : au pire égal au séquentiel (si
REJECTED, le travail QA est jeté et refait après correction — mais rien n'est perdu par rapport à
l'attente séquentielle), au mieux la durée complète de QA est économisée (si APPROVED).

### Déclenchement

- Le `planner` indique dans son plan un champ `qa_parallelizable: true|false` avec une
  justification d'une ligne (voir adaptation `implementation-planner.md`).
- Si pas de Phase Plan (BUGFIX simple, pas de planner dispatché) : le CDP applique une heuristique
  par défaut — `true` pour BUGFIX/HOTFIX à scope minimal sans changement d'architecture ni code de
  concurrence sensible, `false` sinon. Toujours `false` par défaut pour FEATURE sans avis explicite
  du planner.

### Séquence quand `qa_parallelizable: true`

1. Dispatch code-reviewer + test-writer (parallèle, inchangé)
2. test-writer DONE → dispatch qa immédiatement (ne pas attendre code-reviewer)
3. Attendre code-reviewer DONE en parallèle de qa

4a. code-reviewer REJECTED :
    - qa encore en cours → SendMessage qa "ANNULE — retour Dev, résultat invalidé, ignore ta tâche en cours"
    - qa déjà DONE → ignorer son résultat (obsolète, code va changer)
    - Retour Phase Dev (cycle++), comme le flow standard REJECTED

4b. code-reviewer APPROVED / APPROVED WITH RESERVATIONS :
    - Attendre qa DONE s'il n'est pas encore arrivé
    - Traiter le verdict qa normalement (VALIDATED → Doc, NOT VALIDATED → retour Dev cycle++)

### Séquence quand `qa_parallelizable: false`

Flow standard du template (section 5) : Review complète et validée avant dispatch de `qa`.
