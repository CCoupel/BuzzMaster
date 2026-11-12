# Procédure de Test — Import, téléchargement et suppression d'effets lumière (#305, v11.8.0 LOT-1)

**Version** : v11.8.0 (QUALIF)
**Date** : 2026-11-12
**Testeur** : QA (scénarios `AUTO`) ; l'utilisateur pour les scénarios `HUMAIN`
**Contrats** : `contracts/light-effects.md` §11.4 à §11.7
**Maquette** : `docs/mockup/v11.8.0/ui/ambiance_page__effects_catalogue_import_export.html` (§01 à §04)
**Tests automatiques** : `tests/INDEX.md` (lignes #305)

> Les fichiers d'exemple à importer se créent dans `_work/tmp/` (voir chaque scénario). Sauvegarder `data/files/effects/` avant.

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, page Ambiance › Lumière › Effets ouverte
- [ ] Données : dossier `data/files/effects/` vide de fichiers utilisateur (hors `natifs/`)
- [ ] Un navigateur ; pour le scénario 8, au moins un pont ou un groupe d'ampoules configuré

## Scénarios

### Scénario 1 — Télécharger un effet livré et un effet personnalisé

**Validation** : AUTO — **Issues** : #305

**Objectif** : le téléchargement rend le fichier, natifs compris, sans l'en-tête de `natifs/`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `curl -sD- http://<hôte>/api/lighting/effects/pulsation/download` | `200`, `Content-Type: application/yaml; charset=utf-8`, `Content-Disposition: attachment; filename="pulsation.yaml"`, corps = YAML commençant par `id: pulsation`, sans « COPIE DE RÉFÉRENCE » | | |
| 2 | `curl -s .../effects/inconnu/download -o /dev/null -w '%{http_code}'` | `404` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Importer un effet valide (commentaires conservés)

**Validation** : AUTO — **Issues** : #305

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Télécharger `pulsation`, en copier le fichier sous `vague-douce.yaml`, remplacer `id:` par `vague-douce`, ajouter une ligne `# mon commentaire` | Fichier prêt | | |
| 2 | `curl -s -F files=@vague-douce.yaml http://<hôte>/api/lighting/effects/import` | `results[0].status = "imported"`, `stored = "vague-douce.yaml"` | | |
| 3 | `diff vague-douce.yaml data/files/effects/vague-douce.yaml` | Aucune différence (commentaire compris) | | |
| 4 | `curl -s .../api/lighting/effects` | « vague-douce » présent, `origin: "user"` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Conflit : Remplacer et Garder les deux

**Validation** : HUMAIN — **Issues** : #305

**Objectif** : la fenêtre de conflit est compréhensible et chaque choix a l'effet annoncé.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Importer de nouveau `vague-douce.yaml` (bouton « Importer… ») | Fenêtre « Un effet du même identifiant existe déjà » : nom, fichier, « utilisé : … » si utilisé ; boutons Remplacer / Garder les deux / Annuler | | |
| 2 | Annuler | Rien ne change | | |
| 3 | Réimporter, « Garder les deux » | Nouvel effet `vague-douce-2` ; ouvrir `data/files/effects/vague-douce-2.yaml` : seule la ligne `id:` diffère de l'original, le commentaire est intact | | |
| 4 | Modifier la description du fichier à importer, réimporter, « Remplacer » | L'effet `vague-douce` a la nouvelle description, aucun doublon de fichier | | |
| 5 | Importer deux fichiers en une fois dont un seul est en conflit | L'autre est importé immédiatement ; la question ne porte que sur le fichier en conflit, posée après | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Refus d'import (tous les statuts)

**Validation** : AUTO — **Issues** : #305

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Importer un fichier portant `id: pulsation` | `status: "native"`, message « identifiant d'un effet livré » ; rien écrit ; vrai aussi avec `on_conflict=replace` | | |
| 2 | Importer un fichier invalide (variable `gB` non déclarée) | `status: "invalid"`, `errors[].line` renseigné | | |
| 3 | Importer un fichier `.txt` | `400` | | |
| 4 | Importer un fichier de plus de 64 Ko | `status: "too_big"` | | |
| 5 | Importer 21 fichiers | `400` | | |
| 6 | `ls data/files/effects/ data/files/effects/natifs/` | Aucun fichier ajouté dans `natifs/` ni hors du dossier ; aucun fichier partiel | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Résultat d'import lisible (plusieurs fichiers)

**Validation** : HUMAIN — **Issues** : #305

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Choisir en une fois : un fichier valide, un invalide, un portant `id: pulsation` | Panneau au-dessus du catalogue : une ligne par fichier (importé / refusé avec « Ligne N » et le message), lisibles, conformes à la maquette §02 | | |
| 2 | Observer le catalogue | L'effet importé apparaît aussitôt, sans recharger la page ; il est proposé dans les réglages | | |
| 3 | Fermer le panneau | Il disparaît ; il restait affiché jusque-là | | |
| 4 | Déposer un fichier `.yaml` sur le catalogue | Même résultat que le choix de fichiers | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Supprimer et revenir à l'effet d'origine

**Validation** : HUMAIN — **Issues** : #305

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Régler un moment sur `vague-douce`, puis « Supprimer » sur sa carte | Fenêtre citant « utilisé : zone, moment » et le fichier ; propose Télécharger ; wording conforme à la maquette §03 | | |
| 2 | Annuler | Rien ne change | | |
| 3 | Supprimer | Carte disparue, fichier supprimé du serveur ; le moment est signalé comme non résolu (valeur par défaut du moment), le réglage n'a pas été modifié | | |
| 4 | Déposer à la main un `degrade.yaml` modifié (même `id`) dans `data/files/effects/`, recharger | Carte « natif modifié » avec « Revenir à l'effet d'origine » (et sans « Supprimer ») | | |
| 5 | « Revenir à l'effet d'origine » puis confirmer | Fichier supprimé, l'effet livré reprend (étiquette « natif ») | | |
| 6 | Carte d'un effet livré | Ni « Supprimer » ni « Revenir » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Suppression et import en ligne de commande

**Validation** : AUTO — **Issues** : #305

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `curl -s -X DELETE http://<hôte>/api/lighting/effects/pulsation` | `409 {"reason":"native"}`, `natifs/pulsation.yaml` toujours présent | | |
| 2 | `curl -s -X DELETE .../effects/inconnu` | `404` | | |
| 3 | `curl -s -X DELETE .../effects/vague-douce` | `200 {"ok":true,"catalogue":…}`, fichier supprimé | | |
| 4 | Import avec un nom d'envoi `../hors.yaml` (`-F "files=@x.yaml;filename=../hors.yaml"`) | Fichier écrit sous `<id>.yaml` dans `data/files/effects/` uniquement ; rien dans le dossier parent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Essayer sur la salle : choix de la zone

**Validation** : HUMAIN — **Issues** : #305

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Cliquer la liste accolée à « Essayer sur la salle » | Seules les zones ayant au moins un groupe sont proposées | | |
| 2 | Choisir « Animateur » | L'effet se joue sur les ampoules de la zone Animateur, puis l'éclairage revient à la normale | | |
| 3 | Ouvrir la liste d'une autre carte | La dernière zone choisie est retenue ; recharger la page : elle l'est encore | | |
| 4 | Carte d'un effet demandant plus de groupes que la zone | Zone grisée avec « demande N groupes ; la zone … en a M » | | |
| 5 | Forcer l'éclairage sur ON (réglages) | Seule la zone Ambiance est grisée, avec la raison | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux passent
- [ ] Aucun fichier d'effet n'apparaît hors de `data/files/effects/`, jamais dans `natifs/`
- [ ] Aucune régression sur le catalogue (aperçu, « utilisé : … », erreurs de fichiers, rechargement)

## Notes QA

[Espace pour observations]
