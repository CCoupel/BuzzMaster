# Index des maquettes

> Tenu exclusivement par le CDP — convention : `context/COMMON.md` section 14.
> Note : les maquettes historiques (avant v11.1) vivent dans `docs/mockups/` (non indexées).

## Actives
| Composant | Feature | Fichier | Version | Relation |
|-----------|---------|---------|---------|----------|
| admin_game_ardoise_zone | #246 receipt_icon | `v11.2/ui/admin_game_ardoise_zone__receipt_icon.html` | v11.2 | Maquette admin, icône de réception (validée 2026-10-10) ; remplace `reveal_gate` |
| anim_ardoise_list | #246 receipt_icon | `v11.2/ui/anim_ardoise_list__receipt_icon.html` | v11.2 | Maquette anim, icône de réception (validée 2026-10-10) ; remplace `reveal_gate` |
| vplayer_ardoise_keyboard | #246 final_ack | `v11.2/ui/vplayer_ardoise_keyboard__final_ack.html` | v11.2 | Maquette tablette, version révisée pause (validée 2026-10-10) |
| admin_teams | #249 lock_during_question | `v11.2/ui/admin_teams__lock_during_question.html` | v11.2 | Page Équipes verrouillée pendant une question (validée 2026-10-11) |
| tv_ardoise_reveal | #250 adaptive_grid | `v11.2/ui/tv_ardoise_reveal__adaptive_grid.html` | v11.2 | Écran TV REVEAL ARDOISE, grille adaptative (validée 2026-10-11) |
| admin_scores | #252 free_points_tv_toggle | `v11.2/ui/admin_scores__free_points_tv_toggle.html` | v11.2 | Page Scores : nombre libre, interrupteur « Afficher sur la TV » (validée 2026-10-11) ; bandeau : libellé périmé, voir `DECISIONS.md` |
| admin_game_award | #254 compact_block_hints_toggle | `v11.2/ui/admin_game_award__compact_block_hints_toggle.html` | v11.2 | Bloc d'attribution compact, commutateur « Tenir compte des indices » (validée 2026-10-11) ; remplace `compact_block_penalty` |
| admin_game_award | #254 team_or_player_level_hints_toggle | `v11.2/ui/admin_game_award__team_or_player_level_hints_toggle.html` | v11.2 | Attribution équipe ou joueur, commutateur « Tenir compte des indices » (validée 2026-10-11) ; remplace `team_or_player_level_penalty` ; complète `compact_block_hints_toggle` |
| admin_question_editor | #254 qcm_wrong_penalty_default0 | `v11.2/ui/admin_question_editor__qcm_wrong_penalty_default0.html` | v11.2 | Éditeur QCM, pénalité de mauvaise réponse par défaut 0 (validée 2026-10-11) |
| admin_navbar | #253 launch_game_entry | `v11.2/ui/admin_navbar__launch_game_entry.html` | v11.2 | Entrée « Lancer la partie » dans la navbar (validée 2026-10-11) ; complète `admin_new_game__launch_overlay` |
| admin_new_game | #253 launch_overlay | `v11.2/ui/admin_new_game__launch_overlay.html` | v11.2 | Overlay « Lancer la partie » (validée 2026-10-11) ; remplace `purge_popup_details` ; complète `admin_navbar__launch_game_entry` |
| admin_questions_list | #245 rounds_entractes_delete_choice | `v11.2/ui/admin_questions_list__rounds_entractes_delete_choice.html` | v11.2 | Éditeur de manches : création automatique des entractes, fenêtre Conserver / Supprimer aussi / Annuler, « Sans manche » en premier (validée 2026-10-11) ; remplace `rounds_unassigned_first` (chaîne non archivée) |
| game_round_indicator | #245 admin_anim_tv_next_entracte | `v11.2/ui/game_round_indicator__admin_anim_tv_next_entracte.html` | v11.2 | Indicateur de manche en jeu : « à suivre » propose les entractes ; anim : Lancer / Terminer l'entracte (validée 2026-10-11) ; remplace `admin_anim_tv_entractes` (chaîne non archivée) ; complète `rounds_entractes_delete_choice` et `docs/mockups/entracte-programme-214.html` (§04) |
| tv_points_award | #265 multi_team | `v11.2.1/ui/tv_points_award__multi_team.html` | v11.2.1 | TV, attribution de points multi-équipes, feu d'artifice par lots de 6 au plus, grille 3 × 2 (validée 2026-10-12) |
| vplayer_points_award | #265 own_team | `v11.2.1/ui/vplayer_points_award__own_team.html` | v11.2.1 | Tablette, gain de sa propre équipe seulement, animation calée sur le lot de la TV (validée 2026-10-12) ; complète `tv_points_award__multi_team` |
| vplayer_qcm_answer | #248 ack | `v11.2.1/ui/vplayer_qcm_answer__ack.html` | v11.2.1 | Tablette, accusé de réponse QCM / SPEEDY / MEMORY, état « équipe déjà servie » (validée 2026-10-12) ; complète `vplayer_ardoise_keyboard__final_ack` |
| admin_network_overlay | #267 no_network | `v11.2.2/ui/admin_network_overlay__no_network.html` | v11.2.2 | Admin, alerte plein écran « serveur sans réseau » : overlay, bandeau replié, toast de retour (validée 2026-10-13) |
| admin_question_editor | #190 memotion_card_points_rule | `v11.2.2/ui/admin_question_editor__memotion_card_points_rule.html` | v11.2.2 | Éditeur de carte MEMOTION, contrôle « Points » (POINTS_RULE) filtré par type, parité anim/TV (validée 2026-10-13) ; complète `docs/mockups/memotion-card-type-selector-184.html` §2 (historique, non indexée) |
| admin_pickers | #221 category_difficulty_unified | `v11.2.2/ui/admin_pickers__category_difficulty_unified.html` | v11.2.2 | Sélecteurs catégorie / difficulté unifiés (CategoryPicker / DifficultyPicker : mono, multi, filtre), 15 sites, QuizMetaForm exclu (validée 2026-10-13) |
| admin_navbar | #284 #286 ambiance_popover | `v11.4.0/ui/admin_navbar__ambiance_popover.html` | v11.4.0 | Popover d'en-tête « Ambiance » (éclairage corrigé #284 + son ON/OFF persisté), interrupteur général page Ambiance (validée 2026-11-05) ; complète `admin_navbar__launch_game_entry` |
| admin_game_award | #289 selection_lines | `v11.4.0/ui/admin_game_award__selection_lines.html` | v11.4.0 | Bloc d'attribution 3 lignes, compteur « x joueurs · y équipes », bouton sans débordement (validée 2026-11-05) ; complète `compact_block_hints_toggle` et `team_or_player_level_hints_toggle` |
| tv_rafale_started | #282 letter_wheel | `v11.4.0/ui/tv_rafale_started__letter_wheel.html` | v11.4.0 | TV RAFALE alphabétique : composant `Wheel` générique, bandeau ou roue en arc en bas d'écran, même animation (validée 2026-11-05, itération 3) |
| admin_question_editor | #282 rafale_alphabetical | `v11.4.0/ui/admin_question_editor__rafale_alphabetical.html` | v11.4.0 | Section RAFALE : interrupteur « Alphabétique », chips admin/anim avec lettre (validée 2026-11-05) ; vignettes Bandeau/Roue à ajouter (Q15) ; complète `admin_question_editor__memotion_card_points_rule` |
| admin_sounds_manager | #287 #288 countdown_loop | `v11.4.0/ui/admin_sounds_manager__countdown_loop.html` | v11.4.0 | Ambiance › Son : son de décompte, commutateur une fois / en boucle, éditeur de question (validée 2026-11-05) |
| admin_category_manager | #285 rename_color_delete | `v11.4.0/ui/admin_category_manager__rename_color_delete.html` | v11.4.0 | Dialogue « Gérer les catégories » : couleur par pastille, icône custom, renommage, suppression avec fusion historique (validée 2026-11-05, itération 2 ; icône intégrées sans objet, Q17) ; complète `admin_pickers__category_difficulty_unified` |
| admin_sounds_manager | #283 output_diagnostic | `v11.6.0/ui/admin_sounds_manager__output_diagnostic.html` | v11.6.0 | Ambiance › Son : cause lisible de la sortie audio, bannière « Redémarrage requis », ligne de journal (validée 2026-11-09) ; complète `admin_sounds_manager__countdown_loop` |
| admin_question_editor | #235 sound_memory_memotion_entracte | `v11.6.0/ui/admin_question_editor__sound_memory_memotion_entracte.html` | v11.6.0 | Bloc son pour MEMORY, ENTRACTE (boucle, sans « chrono démarre ») et MEMOTION hôte + carte (différé sur le chrono de carte), chip ♪ (validée 2026-11-09) ; complète `admin_question_editor__rafale_alphabetical`, `admin_question_editor__memotion_card_points_rule`, `admin_sounds_manager__countdown_loop` |
| ambiance_settings_panel | #231 #290 global_per_type_tabs | `v11.6.0/ui/ambiance_settings_panel__global_per_type_tabs.html` | v11.6.0 | Composant mutualisé « Réglages d'ambiance », mode global : son coupé, décompte, bruitages en onglets Global / QCM / MEMORY / MEMOTION / RAFALE avec colonnes catalogue, mode d'éclairage par défaut (validée 2026-11-09) ; complète `admin_sounds_manager__countdown_loop`, `admin_navbar__ambiance_popover` |
| ambiance_settings_panel | #290 round_override_files | `v11.6.0/ui/ambiance_settings_panel__round_override_files.html` | v11.6.0 | Même composant, mode surcharge : fenêtre « Ambiance de la manche », tri-états « Hérité / … » **et colonne Fichier** par bruitage et pour le décompte (hérité = fichier global en italique ; fichier de manche = Remplacer / Revenir au fichier global / Tester / Écouter ici, effet immédiat), fichier manquant signalé (validée 2026-11-09, révision 3) ; remplace `ambiance_settings_panel__round_override` ; complète `ambiance_settings_panel__global_per_type_tabs`, `admin_questions_list__round_ambiance_override` |
| admin_questions_list | #290 round_ambiance_override | `v11.6.0/ui/admin_questions_list__round_ambiance_override.html` | v11.6.0 | En-tête de manche : bouton 🎛 Ambiance, badge « Ambiance : N réglages » ; « Sans manche » sans bouton (validée 2026-11-09) ; complète `admin_questions_list__rounds_entractes_delete_choice`, `ambiance_settings_panel__round_override_files` (ex-`round_override`) |
| admin_navbar | #290 ambiance_popover_round_source | `v11.6.0/ui/admin_navbar__ambiance_popover_round_source.html` | v11.6.0 | Popover « Ambiance » : état effectif, étiquette « manche » par réglage imposé, portée du geste direct (validée 2026-11-09) ; complète `admin_navbar__ambiance_popover` |

## Obsolètes
| Fichier | Remplacée par | Version |
|---------|---------------|---------|
| `v11.6.0/ui/ambiance_settings_panel__round_override.html` | `v11.6.0/ui/ambiance_settings_panel__round_override_files.html` | v11.6.0 |
| `v11.2/ui/admin_game_team_award__selection.html` | `v11.2/ui/admin_game_award__compact_block_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__compact_block.html` | `v11.2/ui/admin_game_award__compact_block_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__compact_block_penalty.html` | `v11.2/ui/admin_game_award__compact_block_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__team_or_player_level.html` | `v11.2/ui/admin_game_award__team_or_player_level_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__team_or_player_level_penalty.html` | `v11.2/ui/admin_game_award__team_or_player_level_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_ardoise_zone__reveal_gate.html` | `v11.2/ui/admin_game_ardoise_zone__receipt_icon.html` | v11.2 |
| `v11.2/ui/anim_ardoise_list__reveal_gate.html` | `v11.2/ui/anim_ardoise_list__receipt_icon.html` | v11.2 |
| `v11.2/ui/admin_new_game__purge_popup.html` | `v11.2/ui/admin_new_game__launch_overlay.html` | v11.2 |
| `v11.2/ui/admin_new_game__purge_popup_details.html` | `v11.2/ui/admin_new_game__launch_overlay.html` | v11.2 |

> Chaîne #245 non archivée (aucune de ces maquettes n'est dans `docs/mockup/`) : `admin_questions_list__rounds_unassigned_first` (f′) remplacée par `rounds_entractes_delete_choice` (f″) ; `game_round_indicator__admin_anim_tv_entractes` (g″) remplacée par `admin_anim_tv_next_entracte` (g‴). Étapes intermédiaires f, g, g′ : voir `_work/reports/planner-20261012-223000.md` §3.
