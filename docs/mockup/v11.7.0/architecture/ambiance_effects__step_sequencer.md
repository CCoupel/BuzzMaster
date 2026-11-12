---
mockup:
  composant: ambiance_effects
  feature: step_sequencer
  version: 11.7.0
  type: architecture
  issue: "#223 #225 #299"
  validee_le: 2026-11-11
  complete: []
  remplace: [docs/mockup/v11.7.0/architecture/ambiance_effects__declarative_sequencer.md]
---

# Effets lumière pas à pas — architecture (v11.7.0, révision 4 : trois zones)

Référence normative : `contracts/light-effects.md`. Remplace `ambiance_effects__declarative_sequencer` : les schémas 1 et 2
sont repris et mis à jour (trois zones — ambiance, animateur, public —, groupes numérotés, vitesse, test), le schéma 3 est
réécrit pour le format pas à pas, le schéma 4 est nouveau.

## 1. Du fichier à l'ampoule

```mermaid
flowchart LR
  subgraph Fichiers
    N["Effets natifs<br/>embarqués dans le binaire<br/>assets/effects/*.yaml"]
    R["Copie de référence<br/>data/files/effects/natifs/<br/>(réécrite au démarrage, jamais lue)"]
    U["Effets de l'utilisateur<br/>data/files/effects/*.yaml"]
  end
  subgraph effects["internal/effects (pur)"]
    L["Chargement + contrôle<br/>démarrage, toutes les 2 s, bouton Recharger"]
    C["Catalogue<br/>effets valides + erreurs (fichier, ligne, cause)"]
    RD["Render(effet, contexte, tick)<br/>joue le pas du tick<br/>→ un état par groupe"]
  end
  subgraph ambiance["internal/ambiance"]
    S["Lecteur d'effets<br/>goroutine unique<br/>une trame par zone<br/>compte les ticks du moment"]
    SC["Scene()<br/>scène de salle + états des groupes<br/>de chaque zone"]
  end
  W["lighting.Writer<br/>cadence 250 ms (inchangé)"]
  H["Pilote Hue<br/>un groupe Hue par groupe"]
  B(("Ampoules<br/>zones Ambiance, Animateur, Public<br/>g1, g2… dans chaque zone"))

  N --> L
  N -. copie .-> R
  U --> L
  L --> C --> RD
  CFG["Réglages par moment et par zone<br/>effet + vitesse<br/>global ← manche"] --> S
  G["Jeu en direct<br/>phase, chronomètre, thème, équipe"] --> S
  GR["Groupes numérotés par zone<br/>lighting.groups (rôle = zone)"] --> S
  RD --> S
  S -- "états publiés + NotifyState()" --> SC --> W --> H --> B
  C --> API["API catalogue / aperçu / essai"] --> UI["Ambiance › Lumière › Effets<br/>Tester (zone, vitesse réglée)<br/>Essayer (salle, ×1)"]
```

Un fichier utilisateur de même identifiant qu'un natif le remplace ; le supprimer restaure le natif. Un fichier invalide,
ou écrit dans l'ancien format, est écarté et signalé, sans effet sur les autres ni sur le serveur. La durée réelle d'un
tick est celle du fichier divisée par la vitesse réglée, sans descendre sous 250 ms.

## 2. Qui décide de ce que montre la zone Ambiance (la salle)

```mermaid
flowchart TD
  A{"Éclairage activé<br/>et pont disponible ?"} -- non --> Z["Aucune écriture"]
  A -- oui --> GT{"Essai d'un groupe<br/>en cours ?"}
  GT -- oui --> GTS["Séquence d'essai sur ce groupe<br/>(éteint, blanc, rouge, vert, bleu)"]
  GT -- non --> B{"Mode ON / OFF<br/>ou Flash ?"}
  B -- oui --> O["Forçage manuel (inchangé)<br/>aucun effet de salle"]
  B -- "non (AUTO)" --> T{"Test ou essai d'un effet<br/>sur cette zone ?"}
  T -- oui --> E["Pas de l'effet testé ou essayé"]
  T -- non --> M{"Moment vivant ?"}
  M -- "équipe qui marque" --> SCO["Effet du réglage « score »"]
  M -- "chronomètre" --> CH["Effet du réglage « chrono »"]
  M -- "hors partie, pause, entracte" --> CA["Effet du réglage « calme »"]
  M -- "autre" --> BASE["Scène de salle ordinaire"]
  SCO & CH & CA --> AP{"Effet présent, valide<br/>et assez de groupes ?"}
  AP -- oui --> FR["Pas du tick en cours<br/>un état par groupe"]
  AP -- non --> DEF["Valeur par défaut du moment<br/>(pulsation / aucun effet)"]
```

Les ampoules d'équipe ne passent jamais par ce schéma : elles gardent la couleur de leur équipe.

Les deux autres zones suivent un schéma plus court, sans forçage manuel ni scène de salle :

```mermaid
flowchart TD
  A2{"Éclairage activé<br/>et pont disponible ?"} -- non --> Z2["Aucune écriture"]
  A2 -- oui --> G2{"Essai d'un groupe<br/>ou test d'un effet<br/>sur cette zone ?"}
  G2 -- oui --> E2["Séquence d'essai / pas de l'effet testé"]
  G2 -- non --> M2{"Effet réglé pour le moment vivant,<br/>présent, valide,<br/>assez de groupes dans la zone ?"}
  M2 -- oui --> F2["Pas du tick en cours<br/>un état par groupe de la zone"]
  M2 -- "non, zone Animateur" --> W2["Blanc fixe"]
  M2 -- "non, zone Public" --> N2["Rien n'est envoyé :<br/>les lampes gardent leur état"]
```

ON / OFF et le Flash n'agissent que sur la zone Ambiance. L'interrupteur général et l'arrêt du serveur éteignent les trois
zones. Autour d'un essai ou d'un test sur la zone Public, l'état des lampes est lu sur le pont puis rétabli.

## 3. Ce qu'écrit un fichier d'effet

```mermaid
flowchart LR
  F["Fichier d'effet"] --> ID["Identité<br/>id, nom, description, moments,<br/>groupes_min"]
  F --> H["Horloge<br/>duree_tick_ms, boucle"]
  F --> V["Variables de groupe<br/>gA: g1; +1 (progression)<br/>gC: [g1, g4, g3, g7] (liste)"]
  F --> P["Pas"]
  P --> P1["pre — une fois, au démarrage"]
  P --> P2["pas — la boucle, un pas par tick"]
  P --> P3["dernier — fin de chaque tour"]
  P --> P4["post — à la fin du moment"]
  P2 --> ACT["Un pas = des actions<br/>cible: [couleur, intensité]"]
  ACT --> CIB["Cibles<br/>g0 = toute la zone<br/>g1, g2… = un groupe<br/>gA, gA+1 = variable, voisin"]
  ACT --> COL["Couleurs<br/>#RRGGBB, theme, equipe,<br/>urgence, scene"]
```

## 4. Le déroulement d'un effet

```mermaid
stateDiagram-v2
  [*] --> PRE : début du moment (tick 0)
  PRE --> Pas0 : tick suivant — variables à leur départ
  [*] --> Pas0 : pas de PRE
  Pas0 --> PasSuivant : tick — les variables avancent d'un cran
  PasSuivant --> PasSuivant : tick — un cran
  PasSuivant --> Dernier : après le dernier pas — pas d'avance
  Dernier --> Pas0 : tick — un cran
  PasSuivant --> Pas0 : pas de « dernier » — un cran
  Pas0 --> POST : fin du moment
  PasSuivant --> POST : fin du moment
  Dernier --> POST : fin du moment
  POST --> [*] : sans attendre, recouvert en zones Ambiance et Animateur, tenu en zone Public
```

Un même fichier se joue sur n'importe quelle zone : `g0` et `g1`…`gN` désignent toujours les groupes de la zone jouée. Un
groupe non cité par un pas garde son état. Un pas qui ne vise aucun groupe existant est sauté, sans tick et sans
avance. Les variables rebouclent sur le nombre réel de groupes : le fichier ne connaît pas la taille de la salle.
