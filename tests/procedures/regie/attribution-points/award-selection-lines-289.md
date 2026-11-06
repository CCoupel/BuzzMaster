# Procédure de Test — Bloc d'attribution en 3 lignes (#289)

**Version** : v11.4.0 (LOT-1)
**Date** : 2026-11-05
**Testeur** : utilisateur (HUMAIN)
**Maquette** : `docs/mockup/v11.4.0/ui/admin_game_award__selection_lines.html`

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, page Jeu admin, question révélée (points attribuables)
- [ ] Données : au moins 3 équipes avec plusieurs joueurs, dont deux joueurs d'une même équipe
- [ ] Largeurs d'essai : 1280 px et 1024 px

## Scénarios

### Scénario 1 — Compteur au niveau joueur (#289)

**Validation** : HUMAIN — **Issues** : #289

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Niveau « joueur », aucune sélection | Ligne 2 : « Aucune sélection » ; bouton « Attribuer les points » sans compteur | | |
| 2 | Cocher 1 joueur | « 1 joueur · 1 équipe » (singulier) | | |
| 3 | Cocher un 2ᵉ joueur de la même équipe | « 2 joueurs · 1 équipe » | | |
| 4 | Cocher un joueur d'une autre équipe | « 3 joueurs · 2 équipes » ; mise à jour immédiate à chaque clic | | |
| 5 | Tout désélectionner (icône) | « Aucune sélection » | | |
| 6 | Tout sélectionner (icône) | Compteur = total joueurs · total équipes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Compteur au niveau équipe (#289)

**Validation** : HUMAIN — **Issues** : #289

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Passer au niveau « équipe », cocher 1 équipe | « 1 équipe » | | |
| 2 | Cocher 3 équipes | « 3 équipes » | | |
| 3 | Aucune | « Aucune sélection » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Mise en page sans débordement (#289)

**Validation** : HUMAIN — **Issues** : #289

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 1280 px, sélection quelconque | Ligne 1 : source + « Indices » ; ligne 2 : compteur seul en gras + icônes à droite ; ligne 3 : bouton pleine largeur | | |
| 2 | 1024 px puis 800 px | Aucun retour à la ligne dans le bouton (une seule ligne, ellipse si trop étroit) ; aucun débordement du bloc | | |
| 3 | Sélectionner beaucoup de joueurs (compteur à 2 chiffres) | Libellé du bouton inchangé (pas de compteur dedans) ; mise en page stable | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Pénalité et attribution inchangée (#289)

**Validation** : HUMAIN — **Issues** : #289

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question à pénalité, 2 joueurs sélectionnés | Ligne 1 « Pénalité · −N pts par joueur » ; bouton « Appliquer les pénalités » ; compteur « 2 joueurs · 2 équipes » | | |
| 2 | Cliquer le bouton | Points/pénalités appliqués exactement comme avant (scores joueurs et équipes identiques à v11.3.x) | | |
| 3 | Régression : bouton désactivé sans sélection | Bouton inactif quand « Aucune sélection » | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Compteurs conformes (singulier/pluriel, équipes distinctes, temps réel)
- [ ] Aucun débordement du bouton à 1280/1024/800 px
- [ ] Attribution des points inchangée

## Notes QA

