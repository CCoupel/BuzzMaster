# Procédure de Test — Popup d'aide clé API & Auto-sélection fournisseur IA

**Version** : 6.1.2 (bugfix/config-api-key-help)
**Date** : 2026-08-08
**Testeur** : QA
**Commits** : `38164db` (popup d'aide), `5d29182` (auto-sélection fournisseur)
**Handoff dev** : `_work/handoff/dev-frontend-20260808-164252.md`

> ⚠️ **Contexte important** : les tests automatisés vitest touchant `ConfigPage.jsx`
> (`ConfigPage.test.jsx`, `ConfigPage.ai.test.jsx`, `ConfigPage.apikeyhelp.test.jsx`)
> restent **bloqués indéfiniment** dans certains environnements WSL2 (0% CPU, aucune
> sortie même après 35 min — reproduit aussi bien avant qu'après ce bugfix, donc
> problème d'environnement, pas de régression du code). Voir
> `_work/reports/test-writer-20260808-*.md` pour le détail de l'investigation.
> **Cette procédure manuelle est donc la validation de référence tant que ces tests
> n'ont pas pu être exécutés avec succès** (Windows natif hors interop WSL, ou CI
> Linux natif).

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

## Scénario 2 — Auto-sélection du fournisseur (#5d29182)

**Objectif** : Vérifier que le sélecteur Claude/Groq se désactive/sélectionne automatiquement selon les clés disponibles.

### 2a — Aucune clé enregistrée

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Config vierge (aucune clé), recharger `/config` | Les 2 boutons "Claude (Anthropic)" et "Groq" du sélecteur "Fournisseur" sont **grisés/désactivés** | | |

### 2b — Enregistrement de la clé Groq seule

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 2 | Coller une clé dans "Clé API Groq", cliquer "Enregistrer" | Badge "✅ Clé configurée" sur Groq | | |
| 3 | Observer le sélecteur "Fournisseur" | Bouton "Groq" devient actif (surligné) **automatiquement**, bouton "Claude (Anthropic)" reste désactivé | | |

### 2c — Enregistrement de la clé Claude (les deux clés existent)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 4 | Coller une clé dans "Clé API Claude", cliquer "Enregistrer" | Badge "✅ Clé configurée" sur Claude | | |
| 5 | Observer le sélecteur "Fournisseur" | Bouton "Claude (Anthropic)" devient actif **automatiquement** (Claude prioritaire sur Groq), les 2 boutons sont maintenant cliquables | | |
| 6 | Cliquer manuellement sur "Groq" dans le sélecteur | Groq devient actif — la bascule manuelle reste possible tant que la clé existe | | |

### 2d — Suppression de la clé active

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 7 | Fournisseur actif = Groq (suite étape 6). Cliquer "Supprimer la clé" sur le bloc Groq, confirmer | Badge Groq repasse à "⚠️ Aucune clé". Le sélecteur **bascule automatiquement sur "Claude (Anthropic)"** (seule clé restante) | | |
| 8 | Cliquer "Supprimer la clé" sur le bloc Claude, confirmer | Badge Claude repasse à "⚠️ Aucune clé". **Aucune clé plus disponible** → les 2 boutons du sélecteur redeviennent grisés, le dernier fournisseur sélectionné reste affiché en surbrillance (désactivé) | | |

### 2e — Persistance après rechargement

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 9 | Enregistrer à nouveau la clé Groq uniquement, recharger la page `/config` (F5) | Au chargement, Groq est ré-affiché comme fournisseur actif automatiquement (pas de flash visuel vers un autre état) | | |

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

## Critères de Validation

- [ ] Scénario 1 (popup d'aide) : PASS pour Claude ET Groq, les 3 mécanismes de fermeture (×, clic extérieur, Echap) fonctionnent
- [ ] Scénario 2 (auto-sélection) : PASS sur les 5 sous-cas (aucune clé / une clé / deux clés / suppression / persistance)
- [ ] Scénario 3 (non-régression génération IA) : PASS
- [ ] Aucune régression visuelle sur les autres sections de `/config` (WiFi, Firmware, Néon, etc.)
- [ ] Aucune clé API en clair visible dans le code source de la page (vérifier via "Afficher le code source" — seul `api_key_configured`/`groq_api_key_configured` doit transiter, jamais la clé elle-même)

## Notes QA

- Si possible, relancer les 3 fichiers vitest (`ConfigPage.test.jsx`, `ConfigPage.ai.test.jsx`,
  `ConfigPage.apikeyhelp.test.jsx`) sur un environnement Windows natif (hors WSL) ou une
  CI Linux, et documenter ici le résultat (PASS/FAIL/toujours bloqué) pour lever le doute
  sur l'origine environnementale du blocage.
- `server-go/config.json` local peut contenir une clé Groq réelle laissée par dev-frontend
  lors de la validation de la maquette — **ne jamais commiter ce fichier tel quel**, vider
  le champ `groq_api_key` avant tout commit futur de la config locale.

[Espace libre pour observations complémentaires]
