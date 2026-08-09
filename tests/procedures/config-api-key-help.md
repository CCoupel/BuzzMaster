# Procédure de Test — Popup d'aide clé API, sélecteur de fournisseur IA & tooltip "Générer"

**Version** : 6.1.2 (bugfix/config-api-key-help)
**Date** : 2026-08-08, mise à jour 2026-08-09
**Testeur** : QA
**Commits** : `38164db` (popup d'aide), `1d5de9f` (sélecteur manuel — remplace
l'auto-sélection `5d29182`/`3a595c3`, retirée sur décision CDP suite à un bug critique
trouvé en revue), `adfd576` (tooltip bouton "✨ Générer" de AIGenerateModal)
**Handoffs dev** : `_work/handoff/dev-frontend-20260808-164252.md`,
`_work/handoff/dev-frontend-20260809-105500.md`

> ⚠️ **Contexte important** : les tests automatisés vitest touchant `ConfigPage.jsx`
> (`ConfigPage.test.jsx`, `ConfigPage.ai.test.jsx`, `ConfigPage.apikeyhelp.test.jsx`)
> restent **bloqués indéfiniment** dans certains environnements WSL2 (0% CPU, aucune
> sortie même après 35 min — reproduit aussi bien avant qu'après ce bugfix, donc
> problème d'environnement, pas de régression du code). Voir
> `_work/reports/test-writer-20260808-*.md` pour le détail de l'investigation.
> **Cette procédure manuelle est donc la validation de référence tant que ces tests
> n'ont pas pu être exécutés avec succès** (Windows natif hors interop WSL, ou CI
> Linux natif). Les tests d'`AIGenerateModal` (tooltip, Scénario 4) ne dépendent PAS
> de `ConfigPage` et ne sont pas concernés par ce blocage.
>
> **Mise à jour 2026-08-09** : le Scénario 2 ci-dessous a été entièrement réécrit —
> l'auto-sélection du fournisseur (`pickAutoProvider`, commit `5d29182`) a été
> **retirée** au profit d'une sélection strictement manuelle (commit `1d5de9f`), sur
> décision CDP suite à un bug critique trouvé en revue de code. L'ancienne version de
> ce scénario (boutons grisés selon présence de clé, bascule automatique) ne
> correspond plus au comportement actuel.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Serveur démarré (binaire portable ou `go build && ./server.exe`), accès admin sur `/config`
- [ ] Aucune clé API Claude ni Groq enregistrée au départ (config vierge) — sinon
      utiliser les boutons "Supprimer la clé" pour repartir à zéro avant le Scénario 1
- [ ] Une clé API Claude valide de test (format `sk-ant-...`) et/ou une clé Groq valide
      (format `gsk_...`) — **des valeurs factices suffisent pour les scénarios 1 à 3**
      (le format n'est validé côté serveur qu'à l'enregistrement réel ; pour les
      scénarios de sauvegarde effective, utiliser de vraies clés de test si possible)

---

## Scénario 1 — Popup d'aide clé API (#38164db)

**Objectif** : Vérifier que le bouton "?" ouvre une popup d'aide adaptée au fournisseur, avec liens fonctionnels.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aller dans `/config`, dérouler jusqu'à la section "IA" | Section visible avec 2 blocs "Claude (Anthropic)" et "Groq" | | |
| 2 | Observer le champ "Clé API Claude" | Un bouton rond "?" est présent à droite du libellé | | |
| 3 | Cliquer sur le bouton "?" à côté de "Clé API Claude" | Une popup s'ouvre : titre "Obtenir une clé API Claude (Anthropic)", badge orange "Payant" | | |
| 4 | Observer le contenu de la popup | 3 étapes numérotées : "Créer un compte", "Générer une clé API" (avec mention du moyen de paiement requis), "Coller la clé dans BuzzControl" | | |
| 5 | Cliquer sur le lien "Ouvrir console.anthropic.com ↗" | Nouvel onglet ouvert vers `https://console.anthropic.com` | | |
| 6 | Revenir sur BuzzControl, cliquer sur "Ouvrir console.anthropic.com/settings/keys ↗" | Nouvel onglet ouvert vers `https://console.anthropic.com/settings/keys` | | |
| 7 | Fermer la popup via le bouton "×" | Popup fermée | | |
| 8 | Rouvrir la popup, fermer en cliquant **en dehors** de la boîte (zone sombre) | Popup fermée | | |
| 9 | Rouvrir la popup, appuyer sur la touche **Echap** | Popup fermée | | |
| 10 | Cliquer sur le bouton "?" à côté de "Clé API Groq" | Popup Groq : titre "Obtenir une clé API Groq", badge vert "Gratuit — recommandé", texte "Aucune carte bancaire requise" (pas de mention de moyen de paiement) | | |
| 11 | Vérifier les liens Groq | "Ouvrir console.groq.com ↗" → `https://console.groq.com` ; "Ouvrir console.groq.com/keys ↗" → `https://console.groq.com/keys` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Sélecteur de fournisseur, sélection strictement manuelle (#1d5de9f)

**Objectif** : Vérifier que le sélecteur Claude/Groq est toujours cliquable dans les
deux sens, qu'une seule carte fournisseur est visible à la fois, et qu'aucune
sauvegarde/suppression de clé ne change la sélection active.

### 2a — Aucune clé enregistrée : les 2 boutons restent cliquables

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Config vierge (aucune clé), recharger `/config` | Les 2 boutons "Claude (Anthropic)" et "Groq" du sélecteur "Fournisseur" sont **cliquables** (ni grisés, ni tooltip d'avertissement) | | |
| 2 | Observer la carte affichée sous le sélecteur | **Une seule carte** est visible (celle du fournisseur actif, "Claude (Anthropic)" par défaut) — l'autre fournisseur n'apparaît pas du tout dans la page | | |

### 2b — Bascule manuelle Claude → Groq → Claude

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 3 | Cliquer sur "Groq" dans le sélecteur | La carte Claude disparaît, la carte Groq apparaît (champ clé, badge, bouton "?" dédiés à Groq). Le bouton "Groq" est surligné (actif) | | |
| 4 | Recharger la page (F5) | La carte Groq reste affichée (sélection persistée côté serveur dès le clic, pas besoin de "Enregistrer" séparé) | | |
| 5 | Cliquer sur "Claude (Anthropic)" | La carte Groq disparaît, la carte Claude réapparaît | | |

### 2c — Enregistrer/supprimer une clé ne change JAMAIS la sélection active

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 6 | Fournisseur actif = Claude (suite étape 5), Claude sans clé. Coller une clé Claude, cliquer "Enregistrer" | Badge "✅ Clé configurée" sur la carte Claude — **toujours** la carte Claude affichée, le sélecteur ne bascule PAS vers Groq même si Groq a une clé par ailleurs | | |
| 7 | Cliquer "Supprimer la clé" sur la carte Claude, confirmer | Badge repasse à "⚠️ Aucune clé" — **la carte Claude reste affichée**, le bouton "Claude (Anthropic)" reste actif/surligné dans le sélecteur (pas de bascule vers Groq même si Groq a une clé) | | |
| 8 | Cliquer sur "Groq", coller une clé Groq, cliquer "Enregistrer" | Badge "✅ Clé configurée" sur la carte Groq — la sélection reste sur Groq, aucune bascule automatique vers Claude même si Claude a (ou n'a pas) de clé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Non-régression génération IA (QuestionsPage)

**Objectif** : Vérifier que l'auto-sélection ne casse pas le bouton "✨ Générer via IA" de `QuestionsPage`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avec une clé Groq enregistrée (fournisseur auto-sélectionné = Groq), aller sur `/questions` | Bouton "✨ Générer via IA" actif | | |
| 2 | Lancer une génération courte (ex : 5 questions) | Génération démarre avec le fournisseur Groq (vérifier dans les logs serveur ou le comportement observé — Groq est plus lent, ~10 min/200 questions vs Claude) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Tooltip du bouton "✨ Générer" désactivé (#adfd576)

**Objectif** : Vérifier que le bouton "✨ Générer" (dans la modale de génération IA,
PAS le bouton "✨ Générer via IA" qui ouvre la modale) explique précisément, au
survol, la ou les conditions manquantes quand il est désactivé.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/questions`, avec un fournisseur IA configuré (clé enregistrée), ouvrir la modale "✨ Générer via IA" | Modale ouverte, formulaire de génération affiché | | |
| 2 | Sans avoir choisi de catégorie ni modifié le thème/publics/difficultés de la section Quiz (formulaire vide côté Quiz), survoler le bouton "✨ Générer" (désactivé/grisé) | Une infobulle native apparaît, listant précisément les champs manquants (ex : *"Champ(s) requis manquant(s) : le thème (section Quiz), au moins un public (section Quiz), au moins une difficulté (section Quiz), au moins une catégorie cible"*) | | |
| 3 | Cocher/sélectionner au moins une catégorie cible en bas de la modale | Le bouton reste désactivé (thème/publics/difficultés toujours manquants côté Quiz), mais l'infobulle ne mentionne **plus** la catégorie | | |
| 4 | Décocher tous les types de question (Speedy/QCM/Memory) | L'infobulle inclut désormais "au moins un type de question activé" | | |
| 5 | Renseigner un thème, au moins un public et une difficulté dans la section Quiz, cocher au moins un type et une catégorie | Le bouton "✨ Générer" devient cliquable — **aucune infobulle** ne s'affiche au survol (rien à expliquer) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 1 (popup d'aide) : PASS pour Claude ET Groq, les 3 mécanismes de fermeture (×, clic extérieur, Echap) fonctionnent
- [ ] Scénario 2 (sélecteur manuel) : PASS — boutons toujours cliquables, une seule carte visible à la fois, aucune bascule automatique sur save/clear
- [ ] Scénario 3 (non-régression génération IA) : PASS
- [ ] Scénario 4 (tooltip "✨ Générer") : PASS — message précis et à jour selon les champs manquants, absent quand le formulaire est valide
- [ ] Aucune régression visuelle sur les autres sections de `/config` (WiFi, Firmware, Néon, etc.)
- [ ] Aucune clé API en clair visible dans le code source de la page (vérifier via "Afficher le code source" — seul `api_key_configured`/`groq_api_key_configured` doit transiter, jamais la clé elle-même)

## Notes QA

- Si possible, relancer les 3 fichiers vitest (`ConfigPage.test.jsx`, `ConfigPage.ai.test.jsx`,
  `ConfigPage.apikeyhelp.test.jsx`) sur un environnement Windows natif (hors WSL) ou une
  CI Linux, et documenter ici le résultat (PASS/FAIL/toujours bloqué) pour lever le doute
  sur l'origine environnementale du blocage. `AIGenerateModal.tooltip.test.jsx`
  (Scénario 4) ne dépend pas de `fetch`/`ConfigPage` et n'est a priori pas concerné par
  ce blocage — à confirmer aussi en environnement propre.
- `server-go/config.json` local peut contenir une clé Groq réelle laissée par dev-frontend
  lors de la validation de la maquette — **ne jamais commiter ce fichier tel quel**, vider
  le champ `groq_api_key` avant tout commit futur de la config locale.

[Espace libre pour observations complémentaires]
