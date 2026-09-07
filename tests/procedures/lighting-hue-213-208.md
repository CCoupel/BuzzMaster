# Procédure de Test — Éclairage d'ambiance Hue : affectation par équipe (#213) et conduite manuelle (#208)

**Version** : 10.0.0.x (milestone v10.0.0, Batch 2 — à exécuter une fois ce batch livré en QUALIF)
**Date** : 2026-09-07 (révision : conduite manuelle passée d'un mécanisme d'écrasement par le jeu à un
sélecteur tri-état `ON | AUTO | OFF` tenant indéfiniment, déplacé de `/anim` vers `/admin/ambiance`)
**Issues** : #213 (affectation équipe → ampoule, dégradation), #208 (conduite manuelle sur
`/admin/ambiance` — sélecteur ON/AUTO/OFF + bascule Flash, restitution d'état), correctif d'asymétrie
ENTRACTE regroupé dans T2.1
**Contrats** : `contracts/lighting.md` §10 (SHA `41825c83` — design en vigueur, tenue indéfinie, **pas**
d'annulation automatique par le jeu), `contracts/hue-bridge.md`
**Maquettes** : `docs/mockups/lighting-hue-config-207.html` (page existante), et les maquettes #213/#208
produites en parallèle de cette procédure (`docs/mockups/lighting-team-assignment-213.html`,
`docs/mockups/lighting-priority-208.md`) — si les libellés d'écran définitifs diffèrent de ceux utilisés
ci-dessous, suivre l'écran réel : c'est lui qui fait foi, pas cette procédure.
**Testeur** : **Utilisateur uniquement** — le matériel Hue réel n'est disponible que sur son installation.
`qa` ne peut pas exécuter cette procédure (règle projet `feedback_manual_qa_is_user_role.md`).

---

## Pourquoi cette procédure ne peut pas être automatisée

Le livrable de cette fonctionnalité est **une couleur sur un mur** — invisible à toute suite de tests
automatisée. Le pilote factice (`fake_driver`) valide le protocole émis, jamais ce qui s'allume
réellement. Cinq bugs de rendu avaient déjà traversé review + QA automatisée sur le cycle précédent
(v9.0.0) sans être détectés avant ce type de vérification manuelle — c'est l'angle mort que cette
procédure couvre.

---

## Prérequis

- [ ] Environnement : QUALIF, serveur démarré, build du Batch 2 (#213/#208) installé
- [ ] Un pont Philips Hue réel, sur le même réseau que le serveur, **déjà associé** (procédure
      `docs/mockups/lighting-hue-config-207.html` §03-04, ou page **Ambiance** du menu abeille)
- [ ] **Au moins 4 ampoules Hue** disponibles et allumables/éteignables à la main :
      - 1 ampoule de rôle **`general`**
      - 2 ampoules affectées chacune à une **équipe différente** (ex. "Rouges", "Bleus")
      - 1 ampoule supplémentaire, gardée **non affectée**, pour les scénarios de dégradation
- [ ] Une partie configurée avec **au moins 3 équipes** (pour dépasser volontairement le nombre
      d'ampoules affectées au scénario de dégradation) et des buzzers physiques opérationnels pour au
      moins 2 équipes
- [ ] Accès à la page **Ambiance** de l'admin (`/admin/ambiance`) : colonne d'affectation équipe →
      ampoule, sélecteur d'éclairage général **ON | AUTO | OFF**, bascule **Flash**
- [ ] Pouvoir ouvrir cette page **Ambiance depuis deux postes ou navigateurs différents** en même temps
      (scénario badge multi-poste)
- [ ] Une question au moins de type **ENTRACTE** programmée dans le quiz, en plus d'un accès à l'action
      manuelle ENTRACTE existante (bouton dans `/admin`)
- [ ] Pouvoir couper l'alimentation du serveur BuzzControl à la demande (Ctrl+C, ou arrêt du service)
      et le relancer

> **Noter la couleur affichée à chaque étape avec les yeux, pas avec un outil de mesure.** L'objectif
> est l'expérience en salle, pas la valeur RGB exacte.

---

## Scénario 1 — 🔴 CRITIQUE — Arrêt serveur pendant une ENTRACTE (Hue ET buzzers doivent s'éteindre)

**Objectif** : Vérifier la décision utilisateur du GATE — c'est le **seul** des trois moments de
restitution qui exige une action explicite : à l'arrêt du serveur, tout doit s'éteindre.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une partie, déclencher une ENTRACTE (manuelle ou programmée — indifférent ici) | La salle passe en scène ENTRACTE (blanc chaud, ampoules allumées), les LED des buzzers s'éteignent | | |
| 2 | Vérifier au mur que les ampoules Hue sont bien allumées en blanc chaud | Confirmé visuellement | | |
| 3 | Arrêter le serveur (procédure d'arrêt normale du projet, pas un débranchement brutal du pont) | Le serveur s'arrête | | |
| 4 | Observer les ampoules Hue **dans les secondes qui suivent** | **Toutes les ampoules configurées (general + équipes) s'éteignent** | | |
| 5 | Observer les LED des buzzers | **Tous les buzzers s'éteignent également** | | |
| 6 | Relancer le serveur | Redémarrage normal, aucune erreur liée à l'éclairage dans les logs de démarrage | | |

**Verdict** : [ ] PASS  [ ] FAIL

⚠️ Avant ce correctif, le comportement était : rien ne se passe, la dernière scène reste figée sur les
ampoules. Un FAIL ici — mur qui reste allumé après arrêt — n'est pas un détail, c'est une régression
sur une décision explicite de l'utilisateur.

---

## Scénario 2 — 🔴 CRITIQUE — Ampoule d'équipe éteinte/injoignable ne doit jamais bloquer les autres

**Objectif** : Vérifier la règle de dégradation la plus visible en soirée : une ampoule absente au mur
(éteinte à l'interrupteur, débranchée, ou déplacée hors de portée du réseau Zigbee) ne doit avoir
**aucun impact** sur les autres ampoules configurées.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Partie en cours, au moins 2 équipes avec ampoule affectée | Chaque équipe a sa couleur au mur lors d'un événement qui la concerne (buzz, tour, score) | | |
| 2 | **Couper l'ampoule d'une équipe à l'interrupteur mural** (pas depuis l'app Hue — un vrai coupure physique) | L'ampoule s'éteint évidemment | | |
| 3 | Déclencher un événement concernant **l'équipe dont l'ampoule est coupée** (buzz, tour, score) | Aucune erreur visible côté animateur/admin, aucun ralentissement perceptible | | |
| 4 | Déclencher au même moment un événement concernant **l'autre équipe** (ampoule toujours alimentée) | **Son ampoule réagit normalement**, sans délai ni scintillement anormal | | |
| 5 | Consulter le badge d'état sur la page Ambiance de l'admin | L'état global reste **`ok`** (le pont répond) — ce n'est pas la même chose qu'une ampoule injoignable individuellement ; vérifier si l'écran signale l'ampoule concernée sans faire chuter l'état global | | |
| 6 | Rallumer l'ampoule à l'interrupteur | Elle reprend l'affichage attendu au **prochain** événement qui la concerne (pas nécessairement instantané — c'est le prochain rafraîchissement qui la corrige) | | |

**Verdict** : [ ] PASS  [ ] FAIL

**Résultat de FAIL le plus grave à surveiller** : les *autres* ampoules cessent elles aussi de réagir,
ou la salle entière se fige. C'est exactement ce que la règle de dégradation doit empêcher.

---

## Scénario 3 — Symétrie ENTRACTE manuelle vs programmée (buzzers ET ambiance)

**Objectif** : Le correctif T2.1 unifie les deux déclencheurs du mécanisme ENTRACTE. Les deux voies
doivent produire **exactement le même résultat** — avant ce correctif, seule la voie manuelle éteignait
les LED des buzzers.

### 3a — Voie manuelle (`ENTRACTE_SET`)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, déclencher l'ENTRACTE manuelle (bouton dédié, hors carte question) | La salle passe en scène ENTRACTE (blanc chaud) | | |
| 2 | Observer les LED des buzzers | **Tous les buzzers s'éteignent** | | |
| 3 | Sortir de l'ENTRACTE manuelle | La salle quitte la scène ENTRACTE, retour à l'état de jeu courant | | |

### 3b — Voie programmée (carte ENTRACTE, 7e type de question, #214)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 4 | Lancer la question ENTRACTE programmée préparée dans le quiz | Au **même instant** où la carte devient active (fin du décompte, pas un événement ultérieur — voir Scénario 14), la salle passe en scène ENTRACTE | | |
| 5 | Observer les LED des buzzers | **Tous les buzzers s'éteignent**, identique à 3a — c'est le point corrigé par T2.1 | | |
| 6 | Passer à la question suivante | Sortie d'ENTRACTE, retour normal | | |

**Verdict** : [ ] PASS  [ ] FAIL

⚠️ Avant le correctif : à l'étape 5, les buzzers restaient allumés sur la voie programmée (asymétrie
confirmée bug, cf. décision GATE §1). Un FAIL ici signale que le correctif n'a pas atteint la voie
programmée, ou l'inverse.

---

## Scénario 4 — Tenue du sélecteur (ON/OFF) pendant une partie en cours

**Objectif** : Vérifier la règle en vigueur (`contracts/lighting.md` §10.1.1, SHA `41825c83`) : en
position **ON** ou **OFF**, le mode s'impose à l'éclairage et **tient indéfiniment** — aucun événement
de jeu ne le recouvre, même en pleine partie. Le design intermédiaire d'écrasement automatique (un
instant en vigueur puis explicitement annulé le même jour) **n'a jamais été livré** : ce scénario
vérifie qu'il n'a pas survécu par erreur dans le code.

### 4a — ON tient

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Partie en cours, question STARTED (scène RUNNING au mur, bleu neutre) | Confirmé visuellement | | |
| 2 | Sur `/admin/ambiance`, placer le sélecteur d'éclairage général sur **ON** | Toutes les ampoules pilotées (general + équipes) passent **immédiatement** à pleine intensité, blanc neutre | | |
| 3 | Faire buzzer un joueur | **Aucun changement au mur** : les ampoules restent en ON (blanc plein feu) | | |
| 4 | Enchaîner plusieurs événements de jeu sans toucher au sélecteur (reveal, question suivante, score attribué) | Le mur reste figé sur ON à chaque étape ; le sélecteur affiché à l'écran reste sur **ON** | | |

### 4b — OFF tient

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 5 | Placer le sélecteur sur **OFF** | Toutes les ampoules pilotées s'éteignent immédiatement | | |
| 6 | Faire buzzer un joueur, marquer un point, passer à la question suivante | Les ampoules restent éteintes à chaque étape ; le sélecteur reste affiché sur **OFF** | | |

**Verdict** : [ ] PASS  [ ] FAIL

⚠️ Un FAIL ici (le sélecteur revient tout seul sur AUTO, ou le mur suit un événement de jeu) signale
que le mécanisme d'annulation automatique abandonné a été implémenté par erreur — c'est le défaut le
plus probable si l'implémentation s'est appuyée sur un document périmé.

---

## Scénario 5 — Relâche explicite (retour sur AUTO) — pendant partie et hors partie

**Objectif** : Vérifier que **seul** un geste manuel sur AUTO relâche un mode ON/OFF, que la
re-dérivation est immédiate et reflète l'état de jeu **actuel** (pas l'état d'avant l'activation du
mode), et que AUTO hors partie ne plonge jamais la salle dans le noir.

### 5a — Retour AUTO pendant une partie

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Partie en cours, sélecteur sur **ON** (mur en blanc plein feu) | Confirmé | | |
| 2 | Sans toucher au sélecteur, faire buzzer un joueur de l'équipe "Rouges" (le mur ne bouge pas, cf. Scénario 4) | Mur toujours en blanc plein feu | | |
| 3 | Ramener le sélecteur sur **AUTO** | Le mur bascule **immédiatement** sur la scène correspondant à l'état de jeu **actuel** (couleur de l'équipe qui vient de buzzer) — **pas** la scène qui existait avant l'activation de ON | | |

### 5b — Retour AUTO hors partie (jamais de noir)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 4 | Aucune partie en cours, sélecteur sur **OFF** (mur éteint) | Confirmé éteint | | |
| 5 | Ramener le sélecteur sur **AUTO** | Le mur revient en **blanc chaud praticable** (scène IDLE, `contracts/lighting.md` §8) — **jamais noir**, alors qu'aucune partie ne tourne | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Flash : bascule prioritaire et indépendante du sélecteur

**Objectif** : Vérifier que Flash prime sur la position du sélecteur (y compris OFF) sans jamais la
déplacer, et qu'à son extinction l'éclairage retourne exactement à ce que dit le sélecteur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Placer le sélecteur sur **OFF** (ampoules éteintes) | Confirmé éteint | | |
| 2 | Activer la bascule **Flash** | Les ampoules pilotées **clignotent visiblement**, malgré le OFF actif — Flash n'est pas bloqué par un OFF | | |
| 3 | Observer le sélecteur pendant que Flash clignote | Le sélecteur **affiche toujours OFF** — Flash ne le déplace pas, c'est une couche de diagnostic séparée | | |
| 4 | Désactiver Flash | Les ampoules reviennent **exactement** à l'état du sélecteur — ici éteintes (OFF) | | |
| 5 | Placer le sélecteur sur **AUTO**, en partie en cours, puis activer Flash | Flash clignote par-dessus la scène de jeu courante | | |
| 6 | Désactiver Flash | Retour à la scène de jeu **re-dérivée** depuis l'état vivant (pas une scène figée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Non-persistance du mode au redémarrage du serveur

**Objectif** : Vérifier que le mode (ON/AUTO/OFF, Flash) est un état **serveur en mémoire uniquement** —
jamais écrit sur disque, jamais restauré au redémarrage. Distinct du Scénario 1 (extinction physique à
l'arrêt) : ici on vérifie la position du sélecteur **après** un redémarrage, pas ce qui se passe au
moment de l'arrêt.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Placer le sélecteur sur **OFF** (ou ON) | Ampoules dans l'état correspondant | | |
| 2 | Arrêter le serveur (méthode normale du projet) puis le relancer | Redémarrage normal | | |
| 3 | Rouvrir la page Ambiance après redémarrage | Le sélecteur affiche **AUTO** (position par défaut) — le mode OFF/ON précédent n'a **pas** été restauré | | |
| 4 | Observer le mur | L'éclairage suit l'état de jeu courant (ou la scène IDLE si aucune partie), pas l'état forcé d'avant l'arrêt | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Badge d'état : lisibilité et cohérence entre postes

**Objectif** : Vérifier que le mode courant (et l'état de Flash) est bien un état **serveur**, visible
correctement sur l'écran, y compris depuis un second poste, et qu'aucun onglet fermé ne l'affecte.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la page `/admin/ambiance` sur un premier poste/onglet | Sélecteur affiché sur la position réelle (ex. AUTO) | | |
| 2 | Ouvrir la même page sur un **second poste ou navigateur** | Affiche **la même position** que le premier poste | | |
| 3 | Depuis le premier poste, placer le sélecteur sur **ON** | Le premier poste reflète ON immédiatement | | |
| 4 | Sans rien faire sur le second poste, patienter quelques secondes | Le second poste finit par afficher **ON** lui aussi — c'est un état serveur, pas un état de navigateur | | |
| 5 | Fermer l'onglet du premier poste | Le mode reste **ON** — fermer un onglet n'a aucun effet sur le mode en cours (vérifiable en rouvrant la page Ambiance ailleurs) | | |
| 6 | Activer Flash depuis un poste | L'indicateur Flash reflète l'état actif sur tous les postes qui ont la page ouverte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Affectation équipe → ampoule et cohérence de couleur salle/buzzers

**Objectif** : Vérifier que chaque équipe affectée à une ampoule reçoit **exactement** la même couleur
au mur que sur ses buzzers (règle de réemploi strict de la palette, aucune seconde palette).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur la page Ambiance de l'admin, affecter l'ampoule A à l'équipe "Rouges" et l'ampoule B à l'équipe "Bleus" (colonne d'affectation équipe → ampoule) | Enregistrement réussi, aucune erreur | | |
| 2 | Faire buzzer un joueur de l'équipe "Rouges" | L'ampoule A prend **exactement** la couleur rouge des buzzers de cette équipe (comparer côte à côte buzzer et ampoule) | | |
| 3 | Faire buzzer un joueur de l'équipe "Bleus" | L'ampoule B prend la couleur bleue, identique à ses buzzers ; l'ampoule A revient à l'état neutre | | |
| 4 | Créditer des points à l'équipe "Rouges" | L'ampoule A fait un flash/scène SCORE dans la couleur de l'équipe, **synchronisé** avec l'effet COMET des buzzers (même durée, retour à la normale au même instant) | | |
| 5 | Observer l'ampoule `general` pendant ces étapes | Elle reste sur la scène courante (RUNNING/READY/etc.), **jamais** sur une couleur d'équipe — le zonage la sépare bien des ampoules d'équipe | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Dégradation : moins d'ampoules que d'équipes

**Objectif** : Vérifier qu'une partie à 3 équipes ou plus, avec seulement 2 ampoules affectées, ne
plante rien et ne bloque aucune ampoule.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer une partie à 3 équipes, mais n'affecter une ampoule qu'à 2 d'entre elles | Enregistrement de configuration accepté, aucune erreur bloquante | | |
| 2 | Faire buzzer/marquer un point pour chacune des 2 équipes avec ampoule | Chaque ampoule affectée réagit normalement, indépendamment | | |
| 3 | Faire buzzer/marquer un point pour la **3e équipe** (sans ampoule) | Aucune erreur, aucun blocage des deux autres ampoules ; l'événement de cette équipe ne produit simplement pas d'effet sur une ampoule dédiée (l'ampoule `general`, elle, suit toujours la scène globale) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Dégradation : équipe sans ampoule affectée

**Objectif** : Cas particulier du précédent, isolé pour vérifier spécifiquement qu'une équipe non
affectée n'empêche pas la partie de se dérouler normalement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur la page Ambiance, laisser explicitement une équipe **sans** ampoule affectée (menu "aucune"/vide si l'écran le propose) | Configuration acceptée | | |
| 2 | Jouer un tour concernant cette équipe (buzz, tour actif, score) | Le jeu se déroule normalement côté buzzers/écran ; côté éclairage, aucune ampoule dédiée ne réagit — vérifier qu'aucun message d'erreur n'apparaît côté admin/logs | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 12 — Dégradation : aucune ampoule affectée → retour au comportement "tout en general"

**Objectif** : Vérifier le cas de repli total : si aucune équipe n'a d'ampoule affectée (configuration
`role: "general"` uniquement, comme avant #213), le comportement doit être identique à celui livré par
#207, sans régression.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Retirer toute affectation d'équipe sur la page Ambiance (repasser toutes les ampoules en rôle `general`, ou aucune affectation) | Configuration acceptée | | |
| 2 | Jouer un tour complet (buzz, reveal, score, plusieurs équipes) | Toutes les ampoules `general` suivent la même scène globale, comme avant #213 ; aucune tentative de zonage par équipe | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 13 — Resynchronisation au retour du pont Hue

**Objectif** : Vérifier la décision GATE §3 (`contracts/lighting.md` §10.3) — une perte de connexion au
pont en cours de partie ne touche pas à l'éclairage existant (il reste figé), mais **au retour du
pont**, l'éclairage doit se recalculer sur **ce que l'éclairage doit montrer maintenant** : la scène de
jeu courante en position AUTO, ou **le mode ON/OFF réappliqué tel quel** s'il était engagé — jamais un
état neutre arbitraire.

### 13a — Retour du pont en position AUTO

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Partie en cours, sélecteur sur **AUTO**, scène visible au mur (ex. couleur d'une équipe active) | Confirmé | | |
| 2 | Couper l'accès réseau du pont Hue (débrancher le pont, ou couper son port réseau/Wi-Fi) | Le badge d'état sur la page Ambiance passe à **injoignable** (orange) | | |
| 3 | Faire progresser le jeu pendant la coupure (buzz, question suivante, changement d'équipe active) | Le jeu continue normalement, sans latence perceptible ; les ampoules **restent figées** sur la dernière scène reçue (comportement attendu — pas de tentative d'écriture) | | |
| 4 | Rebrancher/reconnecter le pont | Le badge repasse à **ok** | | |
| 5 | Observer les ampoules **sans déclencher volontairement de nouvel événement** | Elles se **recalculent et s'appliquent** automatiquement pour refléter l'état de jeu **actuel** (celui atteint à l'étape 3), pas un état neutre ni la scène d'avant la coupure | | |

### 13b — Retour du pont avec un mode ON/OFF engagé

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 6 | Placer le sélecteur sur **OFF**, puis couper l'accès réseau du pont | Badge **injoignable** | | |
| 7 | Faire progresser le jeu pendant la coupure | Le jeu continue normalement ; les ampoules **restent dans leur dernier état connu** (ici éteintes, avant même la coupure) | | |
| 8 | Rebrancher/reconnecter le pont | Badge repasse à **ok** | | |
| 9 | Observer les ampoules | Elles reviennent en **OFF** (le mode réappliqué), **pas** la scène de jeu courante — le sélecteur affiche toujours OFF | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 14 — Notification de fin de décompte (ENTRACTE programmée sans délai)

**Objectif** : Vérifier le correctif R1 : sur une ENTRACTE programmée, la salle doit passer en scène
ENTRACTE **dès la fin du décompte de lancement de la carte**, pas seulement au prochain événement sans
rapport.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la question ENTRACTE programmée depuis `/admin` | Le décompte de lancement démarre normalement (identique à toute autre question) | | |
| 2 | Observer la salle **au moment précis où le décompte se termine**, sans rien faire d'autre ensuite | La scène ENTRACTE (blanc chaud) apparaît **immédiatement** à la fin du décompte | | |
| 3 | Ne déclencher **aucun** autre événement de jeu pendant 30 secondes | La scène ENTRACTE reste affichée — elle ne dépend d'aucun événement ultérieur pour apparaître | | |

**Verdict** : [ ] PASS  [ ] FAIL

⚠️ Avant correctif : la salle restait sur la scène READY/RUNNING jusqu'à un événement sans rapport
(ex. un buzz sur une question suivante). Un FAIL ici = régression du correctif T2.1.

---

## Scénario 15 — Parcours complet des scènes (smoke test visuel)

**Objectif** : Dérouler une partie normale de bout en bout, sélecteur sur **AUTO**, et confirmer
visuellement que chaque scène de la table de scènes (`contracts/lighting.md` §8) apparaît au bon moment
sur l'ampoule `general`.

| Étape | Action | Scène attendue (ampoule `general`) | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aucune partie en cours | Blanc chaud, intensité modérée (salle praticable) | | |
| 2 | Partie prête à démarrer (PREPARE/READY/décompte) | Blanc franc, intensité montante — sensation d'attention qui monte | | |
| 3 | Question lancée, personne n'a buzzé | Bleu neutre | | |
| 4 | Un joueur buzze | Couleur de son équipe, pleine intensité | | |
| 5 | Pause générale (admin), sans buzz | Ambre, intensité modérée — visuellement distinct du buzz | | |
| 6 | Réponse révélée, au moins une équipe correcte | Vert | | |
| 7 | Réponse révélée, personne correct | Rouge | | |
| 8 | Tour d'équipe actif (MEMORY/MEMOTION/RAFALE) | Couleur de l'équipe active | | |
| 9 | Points attribués | Flash bref dans la couleur de l'équipe créditée, retour automatique après ~5 secondes | | |
| 10 | ENTRACTE (manuelle ou programmée) | Blanc chaud, intensité réduite — **la salle reste éclairée**, contrairement aux buzzers qui s'éteignent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénarios 1 et 2 (critiques) : **PASS obligatoire** — un FAIL sur l'un des deux bloque la
      validation du milestone, quel que soit le résultat des autres scénarios
- [ ] Scénario 3 : les deux voies ENTRACTE (manuelle et programmée) produisent le même résultat sur
      buzzers ET ambiance
- [ ] Scénario 4 : ON et OFF tiennent **indéfiniment** face aux événements de jeu — aucune annulation
      automatique ne doit être observée
- [ ] Scénario 5 : seul un retour manuel sur AUTO relâche un mode, et AUTO hors partie ne doit jamais
      éteindre la salle
- [ ] Scénario 6 : Flash prime toujours sur le sélecteur sans jamais le déplacer
- [ ] Scénario 7 : aucun mode ON/OFF ne survit à un redémarrage du serveur
- [ ] Scénario 8 : le mode est un état serveur, identique et à jour sur tous les postes ouverts
- [ ] Scénarios 10-12 : aucune situation de dégradation ne bloque, ne plante, ni ne fait disparaître une
      ampoule qui devrait fonctionner
- [ ] Scénario 13 : le retour du pont réapplique le mode courant (ON/OFF réappliqué tel quel, ou la
      scène de jeu re-dérivée si AUTO) — jamais un état neutre arbitraire
- [ ] Aucune régression visible sur les fonctionnalités déjà livrées en QUALIF v10.0.0.6 (#204-#207 :
      découverte, association, sélection/test d'ampoules, indicateur tri-glyphe du menu)

## Notes QA

[Espace pour observations libres, captures/photos des ampoules si utile pour documenter un écart]
