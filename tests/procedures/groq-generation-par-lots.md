# Procédure de Test — Provider gratuit Groq + génération par lots (#137, v6.1.0)

**Version** : v6.1.0 (branche `feature/llm-gratuit-cloud`)
**Date** : 2026-08-05
**Testeur** : QA (le point de contrôle §3 de ce document exige un relecteur francophone)

---

## Contexte de la Feature

Complète `tests/procedures/generateur-ia.md` (#8) — ne le remplace pas, mais **corrige son
scénario 4** : depuis #137, `POST /api/generate-questions` est devenu **asynchrone** (changement
**BREAKING**, `contracts/CHANGELOG.md [20260805b]`). Les étapes 5-6 du Scénario 4 de la procédure
#8 (« la modale bascule sur un spinner… le bouton × est désactivé… aucune fermeture n'a lieu »)
sont **obsolètes pour les deux providers**, Claude compris : la modale n'est plus jamais bloquante,
elle affiche une progression par lots et peut être fermée à tout moment pendant la génération.
Utiliser CE document pour valider le parcours de génération à partir de maintenant.

**Pourquoi ce changement** : le tier gratuit Groq (`openai/gpt-oss-120b`) impose 8 000
tokens/minute — un appel unique de 200 questions est impossible. La génération est donc découpée
en lots séquentiels (défaut 20 questions/lot), espacés d'environ 1 minute, exécutés en tâche de
fond avec progression poussée par WebSocket (`AI_GENERATION_PROGRESS`).

Références :
- Contrat : `contracts/ai-multi-provider.md` (étend `contracts/ai-generation.md`)
- Maquette : `_work/mockups/137-generation-tache-de-fond.md`
- Plan : `_work/reports/planner-20260805-204318-plan-137.md`

**Deux inconnues assumées à l'entrée en QA** (cf. plan, notes) :
1. La qualité du français de `gpt-oss-120b` n'a **jamais été mesurée en amont** — c'est le
   Scénario 3 ci-dessous qui en fait la toute première vérification réelle.
2. Le débit exact de Groq (ce que compte le TPM, le comportement au-delà) n'était pas documenté
   au moment du plan — si la génération échoue systématiquement de façon inattendue, consulter le
   handoff de `dev-backend` (calibration empirique T0.1) avant de conclure à un bug.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `feature/llm-gratuit-cloud`)
- [ ] Accès admin (`/admin` ou `/anim`) → `QuestionsPage` et `ConfigPage`
- [ ] Une clé API Groq **réelle** (gratuite sur https://console.groq.com) — le Scénario 2 (200
      questions) prend ~10 minutes, prévoir le temps
- [ ] Une clé API Claude déjà configurée (héritée de #8), pour les scénarios de non-régression et
      de sélection de provider
- [ ] Deux onglets/postes admin distincts pour le Scénario 5 (job unique)
- [ ] Un moyen de couper le réseau externe du serveur (mode avion / pare-feu) — Scénario 4

---

## Scénarios

### Scénario 1 — Sélection du provider et activation du bouton (CA1, maquette §7)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Paramètres → section IA : vérifier la présence d'un sélecteur « Fournisseur » avec Claude et Groq | Les deux options sont visibles, avec la mention sous Groq : « Gratuit, mais limité en débit : comptez ~10 minutes pour 200 questions. » | | |
| 2 | Retirer la clé Claude (Supprimer la clé), garder Groq configuré, sélectionner « Groq » | Bouton « ✨ Générer via IA » de QuestionsPage **actif** | | |
| 3 | Sélectionner « Claude (Anthropic) » sans clé Claude configurée | Bouton « ✨ Générer via IA » **désactivé**, note « Configurer une clé API… » visible | | |
| 4 | Reconfigurer la clé Claude, sélectionner « Claude » | Bouton réactivé | | |
| 5 | Vérifier qu'aucune des deux clés n'apparaît jamais en clair (champ mot de passe, jamais pré-rempli, badge d'état seulement) | Conforme à #8 pour les deux providers | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Génération complète de 200 questions via Groq (~10 min) — CA3, CA4, CA5, CA6

**Objectif** : le scénario central de #137, à exécuter dans son intégralité.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Provider = Groq, clé configurée. Ouvrir la modale, régler le volume à 200 questions, cocher plusieurs catégories, les 4 types actifs | Formulaire identique à #8 (sliders, blocs Quiz) | | |
| 2 | Cliquer « ✨ Générer » | La modale bascule **immédiatement** (pas d'attente) sur l'état EN COURS : « Génération en cours — lot 1 sur 10 », barre de progression, compteurs à 0 | | |
| 3 | Observer QuestionsPage en arrière-plan (ne pas fermer la modale) pendant le 1er lot | Après ~1 lot, de nouvelles questions apparaissent **dans la liste**, sans rechargement (CA4) | | |
| 4 | Observer l'attente entre deux lots (jusqu'à ~1 min) | Un message « Prochain lot dans Ns… » (ou équivalent) indique que l'attente est normale — l'interface ne doit **jamais** paraître figée ou plantée pendant cette minute | | |
| 5 | **Fermer la modale** (× ou Fermer) après le lot 3 ou 4 | La modale se ferme **sans avertissement bloquant** (changement vs #8) | | |
| 6 | Attendre ~1 minute avec la modale fermée, observer QuestionsPage | De nouvelles questions continuent d'apparaître — **le job continue en tâche de fond** (CA5) | | |
| 7 | **Recharger complètement la page** (F5) pendant que le job tourne encore | Après rechargement, rouvrir « ✨ Générer via IA » | La modale se rouvre directement sur la progression en cours (pas le formulaire), avec le bon numéro de lot — **pas de reprise d'état à deviner** (CA5, contrat §10) | | |
| 8 | Laisser le job aller à son terme (~10 minutes au total depuis l'étape 2) | Modale (si ouverte) ou toast (si fermée) : « ✅ Génération terminée — 200 questions créées » (ou proche, selon écarts/rejets) | | |
| 9 | Compter les questions réellement présentes dans QuestionsPage | Correspond au décompte final annoncé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Point de contrôle qualité française (CA11, maquette §8) — OBLIGATOIRE

> **Ce contrôle remplace un test amont qui a été explicitement écarté au GATE 2.** S'il n'est pas
> fait sérieusement ici, la qualité du français de `gpt-oss-120b` n'est vérifiée **nulle part**
> avant livraison. Doit être fait par un **relecteur francophone**, pas coché par défaut.

Sur un lot **réellement généré via Groq** au Scénario 2 (ou un lot dédié de ~20 questions
variées) :

| Critère | Conforme ? |
|---|---|
| Les énoncés sont en français naturel, sans calque de l'anglais (tournures, ponctuation, majuscules de titre) | [ ] |
| Les mauvaises réponses de QCM sont plausibles — pas absurdes, pas hors sujet, pas manifestement plus courtes/longues que la bonne | [ ] |
| Les paires MEMORY sont des associations réelles et non ambiguës | [ ] |
| Les cartes MEMOTION ont un `RECTO_THEME` cohérent avec la question | [ ] |
| Aucune question tronquée ni recopiée d'un lot à l'autre (anti-doublon intra-job) | [ ] |

**Nom du relecteur francophone** : ________________________
**Verdict** : [ ] PASS  [ ] FAIL — si FAIL, décrire précisément les questions problématiques dans les Notes QA ci-dessous, avec leur ID.

---

### Scénario 4 — Reprise sur échec (CA7, contrat §2-§3)

**Objectif** : un lot en échec ne doit jamais annuler le travail déjà fait.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une génération (Groq ou Claude), laisser 2-3 lots se terminer | Questions visibles dans la liste | | |
| 2 | Couper le réseau externe du serveur pendant ~30 secondes puis le rétablir | Si un seul lot échoue puis le suivant réussit : le job **continue** normalement, aucune question perdue | | |
| 3 | Couper le réseau plus longtemps (au-delà du nombre d'échecs consécutifs configuré, défaut 2) | Le job passe en **ÉCHEC**, message : « ⚠️ Génération interrompue après 2 échecs consécutifs — N questions conservées. » avec N > 0 (les lots déjà réussis restent) | | |
| 4 | Vérifier QuestionsPage | Toutes les questions créées avant l'échec sont bien présentes et correctes | | |
| 5 | Cliquer « Réessayer » | Retour au formulaire, valeurs conservées | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Un seul job à la fois (CA9, contrat §9)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis le poste A, lancer une génération | Job démarre, modale en progression | | |
| 2 | Depuis le poste B (second onglet/admin), ouvrir QuestionsPage et cliquer « ✨ Générer via IA » | La modale s'ouvre **directement sur la progression du job du poste A** (ré-attachement), pas un nouveau formulaire | | |
| 3 | Depuis le poste B, si un formulaire apparaissait malgré tout et qu'on tentait de soumettre | Réponse `409 generation_in_progress` — pas de second job | | |
| 4 | Arrêter le job depuis le poste B (« Arrêter ») | Le job s'arrête, les DEUX postes voient la progression finale (CANCELLED) en même temps | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Arrêt manuel (CA6, contrat §11)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une génération de 100+ questions | Job en cours | | |
| 2 | Après 2-3 lots, cliquer « Arrêter » | L'arrêt prend effet **entre deux lots** (le lot en cours, s'il y en a un en vol, va à son terme) — pas d'interruption brutale au milieu d'un appel | | |
| 3 | Observer le message final | « ⏹ Génération arrêtée — N questions conservées. » avec N > 0 | | |
| 4 | Vérifier QuestionsPage | Les N questions sont bien là, aucune question orpheline/incomplète | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Non-régression du chemin Claude (CA8)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Provider = Claude, générer 20 questions (tient dans le `batch_size` par défaut → 1 seul lot) | Le découpage est **transparent** : le comportement perçu est proche de #8 (rapide, ~1 lot), pas de délai d'une minute artificiel puisqu'un seul lot suffit | | |
| 2 | Comparer un `question.json` généré ici à un `question.json` #8 (avant #137), même type/difficulté | Champs identiques (POINTS chaîne, POINTS_TARGET, TIME chaîne, etc.) | | |
| 3 | Générer un volume plus grand (60+ questions) sur Claude | Le découpage se voit alors (plusieurs lots, progression, délai entre eux) — Claude n'est **plus jamais** un appel unique bloquant, même s'il pourrait tenir sous les anciennes limites de #8 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Sécurité de la seconde clé secrète (CA2)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer une clé Groq, recharger Paramètres | Jamais renvoyée en clair, badge « ✅ Clé configurée » | | |
| 2 | Enregistrer un autre réglage (ex. Effet Néon) sans retoucher la clé Groq | La clé Groq **survit** (même règle que #8 pour Claude) | | |
| 3 | Provoquer une erreur (ex. couper le réseau pendant un lot Groq) et observer `/logs` + la console navigateur | La clé Groq n'apparaît **nulle part**, ni dans un message d'erreur affiché | | |
| 4 | « Supprimer la clé » sur Groq | Efface la clé, bouton « ✨ Générer via IA » se désactive si Groq était le provider actif | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 3 (qualité française) **PASS obligatoire**, fait par un relecteur francophone réel
- [ ] Scénario 2 (parcours complet Groq) va jusqu'au bout sans blocage ni perte de questions
- [ ] Aucune régression sur le chemin Claude (Scénario 7) ni sur les règles de secret (Scénario 8,
      et non-régression du Scénario 1 de `tests/procedures/generateur-ia.md` pour la clé Claude)
- [ ] Un échec ou un arrêt manuel conserve toujours l'acquis, et le dit explicitement (Scénarios 4, 6)
- [ ] Un seul job à la fois, ré-attachement fonctionnel depuis un rechargement de page ou un second
      poste (Scénario 5)

## Notes QA

[Espace pour observations — en particulier, coller ici 2-3 exemples de questions générées via
Groq si le Scénario 3 relève un problème de qualité française, avec leur ID, pour faciliter le
retour dev.]
