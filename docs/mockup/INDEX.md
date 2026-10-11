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

## Obsolètes
| Fichier | Remplacée par | Version |
|---------|---------------|---------|
| `v11.2/ui/admin_game_team_award__selection.html` | `v11.2/ui/admin_game_award__compact_block_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__compact_block.html` | `v11.2/ui/admin_game_award__compact_block_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__compact_block_penalty.html` | `v11.2/ui/admin_game_award__compact_block_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__team_or_player_level.html` | `v11.2/ui/admin_game_award__team_or_player_level_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_award__team_or_player_level_penalty.html` | `v11.2/ui/admin_game_award__team_or_player_level_hints_toggle.html` | v11.2 |
| `v11.2/ui/admin_game_ardoise_zone__reveal_gate.html` | `v11.2/ui/admin_game_ardoise_zone__receipt_icon.html` | v11.2 |
| `v11.2/ui/anim_ardoise_list__reveal_gate.html` | `v11.2/ui/anim_ardoise_list__receipt_icon.html` | v11.2 |
| `v11.2/ui/admin_new_game__purge_popup.html` | `v11.2/ui/admin_new_game__launch_overlay.html` | v11.2 |
| `v11.2/ui/admin_new_game__purge_popup_details.html` | `v11.2/ui/admin_new_game__launch_overlay.html` | v11.2 |
