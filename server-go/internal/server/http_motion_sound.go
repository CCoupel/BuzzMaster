package server

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
)

// Sons de cartes MEMOTION (v11.6.0, #235, contracts/sound.md §14.4,
// http-endpoints.md « POST /questions — sons de cartes MEMOTION »).
//
// handleUploadQuestion reconstruit MOTION_CARDS depuis le payload client : sans
// la fusion ci-dessous, la ré-édition d'une question détruirait silencieusement
// les sons de cartes. La préservation est donc faite ICI, côté serveur, par ID
// de carte — jamais côté frontend. Les clés SOUND* envoyées par le client dans
// le JSON `motion_cards` sont ignorées (seules les parties multipart et
// l'existant sur disque font foi).

// motionCardIDPattern : un ID de carte sert à construire un nom de fichier —
// jamais de séparateur de chemin (les ID générés sont du type "mc-1").
var motionCardIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// motionCardSoundRand tire le suffixe numérique (1000–9999) du nom d'un son de
// carte. Couture de test (M2) : remplaçable pour forcer une collision de nom.
var motionCardSoundRand = func() int { return rand.Intn(9000) + 1000 }

// cardWarning est l'élément de `card_warnings` dans la réponse de POST /questions.
type cardWarning struct {
	CardID  string `json:"card_id"`
	Warning string `json:"warning"`
}

// applyMotionCardSounds traite les champs son de chaque carte de `cards` et
// supprime les sons des cartes disparues. Renvoie les avertissements de durée,
// ou (status, message) non nul si un upload est refusé (tout l'enregistrement
// est alors refusé — l'appelant écrit l'erreur et abandonne).
func applyMotionCardSounds(r *http.Request, id, questionsDir string, cards []map[string]interface{}, existing map[string]interface{}) (warnings []cardWarning, commit func(), rollback func(), status int, msg string) {
	warnings = []cardWarning{}
	prefix := "/question/" + id + "/"

	// Cartes existantes par ID.
	existingByID := map[string]map[string]interface{}{}
	if list, ok := existing["MOTION_CARDS"].([]interface{}); ok {
		for _, ci := range list {
			if c, ok := ci.(map[string]interface{}); ok {
				if cid, _ := c["ID"].(string); cid != "" {
					existingByID[cid] = c
				}
			}
		}
	}

	// Les suppressions de fichiers sont différées jusqu'à l'écriture réussie de
	// question.json (commit) : un refus ultérieur ne doit rien détruire.
	var pending []string
	commit = func() {
		for _, name := range pending {
			if err := os.Remove(filepath.Join(questionsDir, name)); err != nil && !os.IsNotExist(err) {
				LogWarn(game.LogComponentHTTP, "Question %s: failed to remove card sound %s: %v", id, name, err)
			}
		}
	}
	// m2 : fichiers créés par cet appel, supprimés si l'enregistrement échoue ensuite.
	var created []string
	rollback = func() {
		for _, name := range created {
			_ = os.Remove(filepath.Join(questionsDir, name))
		}
	}
	rollbackNow := func() func() { rollback(); return nil }
	removeFile := func(url string) {
		if !strings.HasPrefix(url, prefix) {
			return
		}
		name := filepath.Base(strings.TrimPrefix(url, prefix))
		if name == "" || name == "." || name == "question.json" || strings.ContainsAny(name, `/\`) {
			return
		}
		pending = append(pending, name)
	}

	kept := map[string]bool{}
	questionTime, _ := strconv.Atoi(r.FormValue("time"))

	for _, card := range cards {
		// Jamais de confiance au JSON client pour ces trois clés.
		delete(card, "SOUND")
		delete(card, "SOUND_TIMER_DELAYED")
		delete(card, "SOUND_LOOP")

		cardID, _ := card["ID"].(string)
		if cardID == "" {
			continue
		}
		kept[cardID] = true
		old := existingByID[cardID]
		oldSound, _ := old["SOUND"].(string)

		if !motionCardIDPattern.MatchString(cardID) {
			// Pas de fichier construit à partir d'un ID douteux ; l'existant (s'il y en a un) est conservé.
			if oldSound != "" {
				card["SOUND"] = oldSound
				copyCardSoundFlags(card, old)
			}
			continue
		}

		field := "motion_card_" + cardID + "_sound"
		cleared := r.FormValue(field+"_cleared") == "true"
		sound := ""
		timerDelayed, loop := false, false

		if file, header, err := r.FormFile(field); err == nil {
			raw, rerr := readSoundUpload(file, header.Filename)
			file.Close()
			if rerr != nil {
				return nil, nil, rollbackNow(), rerr.status, fmt.Sprintf("carte %s : %s", cardID, rerr.msg)
			}
			result, verr := audio.ValidateQuestionSound(raw)
			if verr != nil {
				st := http.StatusBadRequest
				if isTooLarge(verr) {
					st = http.StatusRequestEntityTooLarge
				}
				return nil, nil, rollbackNow(), st, fmt.Sprintf("carte %s : %s", cardID, verr.Error())
			}
			// M2 : jamais le nom de l'ancien son (il serait écrasé puis supprimé au commit).
			var fileName string
			for {
				fileName = fmt.Sprintf("motion_%s_sound_%d.wav", cardID, motionCardSoundRand())
				if prefix+fileName != oldSound {
					break
				}
			}
			if werr := os.WriteFile(filepath.Join(questionsDir, fileName), audio.BuildCanonicalWAV(result.PCM), 0644); werr != nil {
				LogError(game.LogComponentHTTP, "Question %s: failed to write card sound %s: %v", id, fileName, werr)
				return nil, nil, rollbackNow(), http.StatusInternalServerError, "échec d'écriture du fichier son de carte"
			}
			created = append(created, fileName)
			if oldSound != "" {
				removeFile(oldSound)
			}
			sound = prefix + fileName
			timerDelayed = r.FormValue(field+"_timer_delayed") == "true"
			loop = r.FormValue(field+"_loop") == "true"
			if !timerDelayed && questionTime > 0 && result.Duration > time.Duration(questionTime)*time.Second {
				warnings = append(warnings, cardWarning{CardID: cardID, Warning: fmt.Sprintf(
					"ce son dure %.1f s, plus longtemps que le temps de réponse (%ds) — en mode simultané, la carte sera déjà expirée avant la fin du son",
					result.Duration.Seconds(), questionTime)})
			}
		} else if cleared {
			if oldSound != "" {
				removeFile(oldSound)
			}
		} else if oldSound != "" {
			// Préservation par ID de carte : son et réglages repris de l'existant ;
			// un champ de réglage explicitement présent ("true"/"false") le remplace.
			sound = oldSound
			timerDelayed, _ = old["SOUND_TIMER_DELAYED"].(bool)
			loop, _ = old["SOUND_LOOP"].(bool)
			if v := r.FormValue(field + "_timer_delayed"); v == "true" || v == "false" {
				timerDelayed = v == "true"
			}
			if v := r.FormValue(field + "_loop"); v == "true" || v == "false" {
				loop = v == "true"
			}
		}

		if sound != "" {
			card["SOUND"] = sound
			if timerDelayed {
				card["SOUND_TIMER_DELAYED"] = true
			}
			if loop {
				card["SOUND_LOOP"] = true
			}
		}
	}

	// Cartes disparues : leur fichier son disparaît avec elles.
	for cid, old := range existingByID {
		if !kept[cid] {
			if s, _ := old["SOUND"].(string); s != "" {
				removeFile(s)
			}
		}
	}
	return warnings, commit, rollback, 0, ""
}

func copyCardSoundFlags(card, old map[string]interface{}) {
	if v, _ := old["SOUND_TIMER_DELAYED"].(bool); v {
		card["SOUND_TIMER_DELAYED"] = true
	}
	if v, _ := old["SOUND_LOOP"].(bool); v {
		card["SOUND_LOOP"] = true
	}
}

type uploadReadError struct {
	status int
	msg    string
}

// readSoundUpload lit un fichier .wav envoyé (plafond brut 4 × canonique).
func readSoundUpload(file io.Reader, filename string) ([]byte, *uploadReadError) {
	if !strings.EqualFold(filepath.Ext(filename), ".wav") {
		return nil, &uploadReadError{http.StatusBadRequest, "le son doit être un fichier .wav"}
	}
	raw, err := io.ReadAll(io.LimitReader(file, audio.MaxRawQuestionSoundBytes+1))
	if err != nil {
		return nil, &uploadReadError{http.StatusBadRequest, "lecture du fichier son échouée"}
	}
	if int64(len(raw)) > audio.MaxRawQuestionSoundBytes {
		return nil, &uploadReadError{http.StatusRequestEntityTooLarge, fmt.Sprintf("ce fichier pèse plus que la limite de %d Mio pour un son de question", audio.MaxRawQuestionSoundBytes>>20)}
	}
	return raw, nil
}

func isTooLarge(err error) bool { return errors.Is(err, audio.ErrTooLarge) }
