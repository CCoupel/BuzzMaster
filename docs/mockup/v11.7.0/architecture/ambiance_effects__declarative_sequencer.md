---
mockup:
  composant: ambiance_effects
  feature: declarative_sequencer
  version: 11.7.0
  type: architecture
  issue: "#223 #225 #299"
  validee_le: 2026-11-11
  complete: []
  remplace: []
---

# Effets lumière déclaratifs — architecture (v11.7.0)

Référence normative : `contracts/light-effects.md`.

## 1. Du fichier à l'ampoule

```mermaid
flowchart LR
  subgraph Fichiers
    N["Effets natifs<br/>embarqués dans le binaire<br/>assets/effects/*.yaml"]
    R["Copie de référence<br/>data/files/effects/natifs/<br/>(réécrite au démarrage, jamais lue)"]
    U["Effets de l'utilisateur<br/>data/files/effects/*.yaml"]
  end
  subgraph effects["internal/effects (pur, nouveau)"]
    L["Chargement + validation<br/>démarrage, toutes les 2 s, bouton Recharger"]
    C["Catalogue<br/>effets valides + erreurs (fichier, ligne, cause)"]
    RD["Render(effet, contexte, temps)<br/>→ trame : un état par groupe"]
  end
  subgraph ambiance["internal/ambiance (existant)"]
    S["Lecteur d'effets<br/>goroutine unique, tic 100 ms"]
    SC["Scene()<br/>scène de salle + trame"]
  end
  W["lighting.Writer<br/>cadence 250 ms (inchangé)"]
  H["Pilote Hue<br/>groupes buzzmaster-general-n"]
  B(("Ampoules<br/>par groupe"))

  N --> L
  N -. copie .-> R
  U --> L
  L --> C --> RD
  CFG["Réglages<br/>effet par moment<br/>global ← manche"] --> S
  G["Jeu en direct<br/>phase, chronomètre, thème, équipe"] --> S
  GR["Groupes d'ambiance<br/>lighting.groups"] --> S
  RD --> S
  S -- "trame publiée + NotifyState()" --> SC --> W --> H --> B
  C --> API["API catalogue / aperçu / essai"] --> UI["Ambiance › Lumière › Effets"]
```

Un fichier utilisateur de même identifiant qu'un natif le remplace ; le supprimer restaure le natif. Un fichier invalide
est écarté et signalé, sans effet sur les autres ni sur le serveur.

## 2. Qui décide de ce que montre la zone générale

```mermaid
flowchart TD
  A{"Éclairage activé<br/>et pont disponible ?"} -- non --> Z["Aucune écriture"]
  A -- oui --> B{"Mode ON / OFF<br/>ou Flash ?"}
  B -- oui --> O["Forçage manuel (inchangé)<br/>aucun effet"]
  B -- "non (AUTO)" --> T{"Essai d'un effet<br/>en cours ?"}
  T -- oui --> E["Trame de l'effet essayé"]
  T -- non --> M{"Moment vivant ?"}
  M -- "équipe qui marque" --> SCO["Effet du réglage « score »"]
  M -- "chronomètre" --> CH["Effet du réglage « chrono »"]
  M -- "hors partie, pause, entracte" --> CA["Effet du réglage « calme »"]
  M -- "autre" --> BASE["Scène de salle ordinaire"]
  SCO & CH & CA --> AP{"Effet présent, valide<br/>et assez de groupes ?"}
  AP -- oui --> FR["Trame de l'effet<br/>un état par groupe"]
  AP -- non --> DEF["Valeur par défaut du moment<br/>(pulsation / aucun effet)"]
```

Les ampoules d'équipe ne passent jamais par ce schéma : elles gardent la couleur de leur équipe.

Le **groupe animateur** ne passe pas non plus par ce schéma : ni forçage manuel, ni essai, ni effet de salle. Le lecteur lui
calcule sa propre trame (réglage « effets du groupe animateur », même moment vivant) ; sans effet choisi, il reste en blanc fixe.

## 3. Ce qu'un fichier d'effet peut exprimer

```mermaid
flowchart LR
  F["Fichier d'effet"] --> ID["Identité<br/>id, nom, description, moments"]
  F --> LK["Apparences nommées<br/>couleur (scène, thème, équipe, urgence, #RRGGBB)<br/>intensité, fondu"]
  F --> P["Un motif"]
  F --> ST["Étapes<br/>variantes selon le temps restant<br/>ou le nombre de groupes"]
  P --> U1["uniform — toute la salle alterne"]
  P --> U2["shift — un bloc se déplace de groupe en groupe"]
  P --> U3["gauge — jauge au prorata du chronomètre"]
  P --> U4["random — tirage de groupes"]
  P --> U5["gradient — dégradé fixe ou qui se propage"]
```

Créer un effet = combiner un motif, des apparences et des étapes dans un fichier. Ajouter un **motif** nouveau demande une
évolution du serveur.
