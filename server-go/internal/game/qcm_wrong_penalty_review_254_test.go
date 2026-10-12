package game

import (
	"encoding/json"
	"testing"
)

// #254 review: an invalid QCM_WRONG_PENALTY reads as 0 and NEVER fails the question's decoding.
func TestQuestion_InvalidQCMWrongPenaltyDecodesToZeroWithoutError(t *testing.T) {
	cases := map[string]int{
		`-2.5`:    0,
		`-5.0`:    0,
		`"-5"`:    0,
		`"abc"`:   0,
		`null`:    0,
		`true`:    0,
		`[1]`:     0,
		`5`:       0,    // positive
		`-101`:    0,    // out of range
		`-100`:    -100, // bounds are valid
		`0`:       0,
		`-3`:      -3,
		`1e2`:     0,
		`{"a":1}`: 0,
	}
	for raw, want := range cases {
		var q Question
		doc := `{"ID":"q","QUESTION":"?","ANSWER":"a","TYPE":"QCM","QCM_WRONG_PENALTY":` + raw + `}`
		if err := json.Unmarshal([]byte(doc), &q); err != nil {
			t.Errorf("QCM_WRONG_PENALTY=%s : le chargement ne doit jamais échouer (%v)", raw, err)
			continue
		}
		if int(q.QCMWrongPenalty) != want {
			t.Errorf("QCM_WRONG_PENALTY=%s -> %d, attendu %d", raw, q.QCMWrongPenalty, want)
		}
	}
}

func TestQuestion_ValidQCMWrongPenaltyRoundTrips(t *testing.T) {
	q := Question{ID: "q", Type: QuestionTypeQCM, QCMWrongPenalty: -4}
	b, _ := json.Marshal(q)
	var back Question
	if err := json.Unmarshal(b, &back); err != nil || back.QCMWrongPenalty != -4 {
		t.Fatalf("round trip : %v %d (%s)", err, back.QCMWrongPenalty, b)
	}
}
