# Contraintes de conception

> Contraintes durables issues des refus/corrections de l'utilisateur, par composant.
> Le planner les respecte, QA les vérifie. Tenu par le CDP.

## (toutes maquettes) — règle de dispatch
- Une maquette validée se dispatche dans **tout son périmètre**, pas seulement l'issue nominale : le planner
  liste chaque issue couverte par la maquette et le CDP les dispatche ensemble. _(v11.1.0 : le logo #239 a été
  livré seul alors que la maquette validée couvrait aussi #238.)_

## vplayer_ardoise_keyboard
- En PAUSE (et plus généralement hors saisie), aucun message n'invite le joueur à continuer ou à écrire : afficher l'état de jeu (« Jeu en pause ») et le statut de la réponse (envoi en cours / reçue), rien d'autre. _(v11.2.0, final_ack, #246)_
- Clavier verrouillé et vide : texte indicatif « Aucune réponse », jamais « Votre réponse… ». _(v11.2.0, final_ack, #246)_

## admin_game_ardoise_zone
- L'état de réception d'une réponse est une icône à gauche du texte (colonne fixe), jamais une ligne de pastille supplémentaire : la carte ne grandit pas selon l'état. _(v11.2.0, receipt_icon, #246)_
- La légende de l'en-tête (« ✓ reçue · ⏳ en attente · ⚠ hors ligne ») reste la référence des icônes. _(v11.2.0, receipt_icon, #246)_

## anim_team_award
- Attribution groupée sur l'anim : non retenue en v11.2 (#244/#251 admin seulement). Maquette brouillon `anim_team_award__selection` non archivée. _(v11.2.0, #244/#251)_

## admin_new_game
- Cocher « Joueurs des tablettes » coche et verrouille « Scores » : on ne peut pas supprimer les joueurs en gardant les scores. _(v11.2.0, purge_popup_details, #243, D2)_
- L'option « Questions » (remettre toutes les questions à jouer) est cochée par défaut ; décochée, les questions déjà jouées le restent. _(v11.2.0, purge_popup_details, #243, D1)_

## admin_scores
- Le bandeau « Correction cachée » dit que le classement de la TV est mis à jour sans animation (décision utilisateur 2026-10-11). L'ancien libellé de la maquette (« … se mettra à jour au prochain affichage normal ») est périmé. _(v11.2.0, free_points_tv_toggle, #252)_

## admin_game_award
- Le commutateur « Tenir compte des indices » n'apparaît que s'il y a eu des indices (QCM avec indices activés, au moins un indice révélé, montant > 0). Il est activé par défaut, remis à activé à chaque question ou phase, et n'est modifié ni par les clics sur les réponses ni par la saisie. _(v11.2.0, compact_block_hints_toggle / team_or_player_level_hints_toggle, #254)_
- La ligne 1 du bloc est liée au commutateur. Activé : « barème de la question, par équipe » ; avec une valeur saisie à la main : « montant saisi, ajusté selon les indices de chaque équipe ». Désactivé : montant uniforme. _(v11.2.0, #254)_
- La pénalité de mauvaise réponse est toujours uniforme : le commutateur est masqué et les indices sont sans effet. _(v11.2.0, #254)_

## admin_questions_list
- « Sans manche » se joue en premier. _(v11.2.0, rounds_entractes_delete_choice, #245)_
- Supprimer une manche ouvre une fenêtre Conserver / Supprimer aussi / Annuler : les entractes suivent le même sort que les questions. _(v11.2.0, rounds_entractes_delete_choice, #245)_
- Le repli des manches est mémorisé par navigateur. _(v11.2.0, rounds_entractes_delete_choice, #245)_

## game_round_indicator
- Une entracte est proposée par « à suivre » comme toute entrée. _(v11.2.0, admin_anim_tv_next_entracte, #245)_
- L'anim peut terminer une entracte ; déclencher une entracte manuelle reste réservé à l'admin. _(v11.2.0, admin_anim_tv_next_entracte, #245)_
- Les compteurs de manche ne comptent pas les entractes ; le compteur global de l'anim les compte. _(v11.2.0, admin_anim_tv_next_entracte, #245)_

## tv_points_award
- Lots de 6 équipes au plus, grille d'arrivée 3 × 2 sans recouvrement. _(v11.2.1, multi_team, #265, 2026-10-12)_
- Accélération complète au-delà de 3 lots. _(v11.2.1, multi_team, #265, 2026-10-12)_
- Noms d'équipe tronqués par « … ». _(v11.2.1, multi_team, #265, 2026-10-12)_

## vplayer_points_award
- Seul le gain de l'équipe du joueur est affiché. _(v11.2.1, own_team, #265, 2026-10-12)_
- L'animation est calée sur le lot de la TV. _(v11.2.1, own_team, #265, 2026-10-12)_

## vplayer_qcm_answer
- « ✓ » seulement après accusé du serveur. _(v11.2.1, ack, #248, 2026-10-12)_
- Aucune invitation à réessayer. _(v11.2.1, ack, #248, 2026-10-12)_
- État « équipe déjà servie » présent. _(v11.2.1, ack, #248, 2026-10-12)_
- Renvoi toutes les 4 s, plafond de 30 envois. _(v11.2.1, ack, #248, 2026-10-12)_

## admin_network_overlay
- Critère « sans réseau » : aucune IPv4 utilisable hors boucle locale, APIPA et **interfaces virtuelles connues** (vEthernet, Virtual, WSL, Hyper-V, VMware, VirtualBox, docker*, br-*, veth*, virbr*, tun*, tap*, wg*, lxc*, vmnet*, utun*) — liste best-effort documentée, jamais exhaustive. _(v11.2.2, no_network, #267, 2026-10-13)_
- « Continuer quand même » replie l'overlay en bandeau rouge persistant jusqu'au retour du réseau ; l'overlay revient à chaque nouvelle perte ; jamais de fermeture définitive. _(v11.2.2, no_network, #267, 2026-10-13)_
- Watchdog 30 s conservé ; aucun champ de plateforme ni conseil adapté à l'OS : texte unique. _(v11.2.2, no_network, #267, 2026-10-13)_
- Titre de l'overlay « Le serveur n'a pas de réseau disponible » ; **aucune mention de plateforme** dans le texte (la phrase « Sur un Raspberry Pi, vérifiez que le point d'accès est démarré. » est retirée, Q3 précisée). _(v11.2.2, no_network, #267, 2026-10-14)_
- Admin seulement (toutes les pages `/admin/*`) ; rien sur TV, anim, tablette. _(v11.2.2, no_network, #267, 2026-10-13)_

## admin_question_editor
- Barème `POINTS_RULE` d'une carte MEMOTION : options filtrées par type — binaire (SPEEDY, QCM) : Étoiles (défaut) / Montant fixe ; progression (MEMORY, RAFALE) : Prorata des étoiles (défaut) / Étoiles pleines / Montant fixe / Par unité. Le serveur refuse le reste. _(v11.2.2, memotion_card_points_rule, #190, 2026-10-13)_
- Le défaut choisi n'écrit jamais `POINTS_RULE` sur la carte. _(v11.2.2, memotion_card_points_rule, #190, 2026-10-13)_
- Parité du montant selon `POINTS_RULE` limitée aux surfaces qui affichent déjà un montant (zoom anim, bandeau anim, bouton de crédit anim, panneau admin GamePage) ; les grilles anim (`AnimMotionGrid`) et TV (`PlayerDisplay`) restent en ★ seules, sans montant. _(v11.2.2, memotion_card_points_rule, #190, décision utilisateur 2026-10-15)_
- Les cartes QCM d'une question MEMOTION n'ont pas d'indices : les contrôles d'indices sont retirés de l'éditeur de carte et le moteur n'en déclenche aucun pendant une carte (principe : pas d'indice sans effet sur les points ; option « prise en compte au crédit » écartée). _(v11.2.2, LOT-6, décision utilisateur 2026-10-15)_
- Éditeur de carte MEMOTION : la ligne de lecture du son de carte s'intitule **« Lecture de la carte »** (et non « Lecture » comme la maquette §03) pour la distinguer de la ligne « Lecture » du son d'hôte ; blocs « Son de la manche » (hôte MEMOTION) et « Son de l'entracte » (ENTRACTE). _(v11.6.0, sound_memory_memotion_entracte, #235, revue LOT-2 point 8, 2026-11-09)_

## admin_pickers
- Unification des sélecteurs catégorie / difficulté : tous les sites **sauf QuizMetaForm** (difficultés textuelles Facile…Expert, publics, langue — autre domaine). _(v11.2.2, category_difficulty_unified, #221, 2026-10-13)_
- Chaque site indique explicitement « une seule » ou « plusieurs possibles » ; ✓ sur les chips multiples, radio en sélection unique. _(v11.2.2, category_difficulty_unified, #221, 2026-10-13)_
- La « Valeur de la carte » MEMOTION garde son libellé : ce n'est pas une difficulté de tirage. _(v11.2.2, category_difficulty_unified, #221 ; origine retour QUALIF v9.0.0.4 C5)_


## admin_navbar
- Le popover d'en-tête s'appelle « Ambiance » et regroupe éclairage (ON/AUTO/OFF + Flash) et son (ON/OFF, jamais d'AUTO) ; le son coupé est persisté et toujours visible dans l'en-tête. _(v11.4.0, ambiance_popover, #286, 2026-11-05)_
- Tout popover de la barre porte `.navbar-popover` : ancré à gauche, `white-space: normal`, largeur bornée au viewport. _(v11.4.0, #284, 2026-11-05)_

## ambiance_page
- L'interrupteur général « Éclairage d'ambiance activé » est persisté ; éteint, aucune commande n'atteint les ampoules. _(v11.4.0, #286, 2026-11-05)_

## admin_game_award
- Le bouton d'attribution ne porte jamais de compteur et tient sur une ligne ; le compteur « x joueurs · y équipes » / « y équipes » / « Aucune sélection » occupe sa propre ligne. _(v11.4.0, selection_lines, #289, 2026-11-05)_

## tv_rafale_started (composant Wheel)
- Option A « roue circulaire complète » rejetée. Deux présentations : bandeau (défaut) et roue en arc, choisies par manche RAFALE, toujours en bas de l'écran. _(v11.4.0, letter_wheel, #282, 2026-11-05)_
- Même animation pour les deux présentations : pointeur fixe, contenu qui glisse (pas de 0,6 s). _(v11.4.0, #282, 2026-11-05)_
- Pas de ligne A→Z supplémentaire dans l'encart question : le bandeau ou la roue suffit, homogène avec la Roue de la Fortune. _(v11.4.0, #282, 2026-11-05)_
- La roue doit rester lisible après un nombre quelconque de mouvements (position normalisée modulo n). _(v11.4.0, #282, 2026-11-05)_

## admin_question_editor
- Tri alphabétique RAFALE : accents, casse et articles initiaux ignorés ; une lettre à la fois, cyclique A→Z. _(v11.4.0, rafale_alphabetical, #282, 2026-11-05)_

## admin_sounds_manager
- Son de décompte joué au début de chaque chronomètre (global, question RAFALE, carte MEMOTION), rejoué si « en boucle » ; jamais si la question a un média audio pendant le chrono. _(v11.4.0, countdown_loop, #287, 2026-11-05)_
- Commutateur une fois / en boucle réservé au média de question et au son de décompte, jamais aux bruitages. _(v11.4.0, #288, 2026-11-05)_

## admin_category_manager
- La couleur se choisit en cliquant la pastille (palette), jamais par un champ séparé. _(v11.4.0, rename_color_delete, #285, 2026-11-05)_
- L'icône ne se téléverse que sur une catégorie personnalisée ; les intégrées gardent leur emoji et ne sont jamais supprimables. _(v11.4.0, #285, 2026-11-05)_
- Supprimer une catégorie utilisée demande « réaffecter à Sans catégorie » ou « supprimer aussi » ; les points d'historique sont regroupés dans « Sans catégorie » (sauvegarde + confirmation). _(v11.4.0, #285, 2026-11-05)_
