// #285 (LOT-4, B7) — CATEGORIES_CHANGED : type de changement par route et charge = GET /api/categories.
package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCategories285Broadcast_ChangeTypePerRoute_AndPayloadEqualsGet(t *testing.T) {
	s, _ := dev285Server(t)
	var got []CategoryChange
	var payloads []json.RawMessage
	s.OnCategoriesChanged = func(c CategoryChange) {
		got = append(got, c)
		p, err := s.CategoriesChangedPayload(c)
		if err != nil {
			t.Fatal(err)
		}
		payloads = append(payloads, p)
	}

	dev285Create(t, s, "Cinema", "#0ea5e9")
	if rec := dev285JSON(s, "PUT", "/api/categories/CINEMA", `{"color":"#ff0000"}`); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec := dev285JSON(s, "PUT", "/api/categories/CINEMA", `{"name":"Ciné","color":"#00ff00"}`); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	body, ct := dev285Multipart(t, nil, "i.png", dev285PNG)
	if rec := dev285Do(s, "POST", "/api/categories/CINEMA/icon", body, ct); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec := dev285Do(s, "DELETE", "/api/categories/CINEMA", nil, ""); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	// écriture refusée (409 partie en cours / 400) : aucune diffusion
	before := len(got)
	if rec := dev285JSON(s, "PUT", "/api/categories/CINEMA", `{"color":"#zzz"}`); rec.Code == 200 {
		t.Fatal("attendu un refus")
	}
	if len(got) != before {
		t.Error("une écriture refusée ne doit pas diffuser")
	}

	want := []CategoryChange{
		{CategoryChangeCreated, "CINEMA"}, {CategoryChangeColored, "CINEMA"}, {CategoryChangeRenamed, "CINEMA"},
		{CategoryChangeIcon, "CINEMA"}, {CategoryChangeDeleted, "CINEMA"},
	}
	if len(got) != len(want) {
		t.Fatalf("changements = %+v, attendu %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("changement %d = %+v, attendu %+v", i, got[i], want[i])
		}
	}

	// charge du dernier message (deleted) : clé absente, forme du contrat, liste = GET
	var msg struct {
		Categories json.RawMessage            `json:"CATEGORIES"`
		Change     struct{ TYPE, KEY string } `json:"CHANGE"`
		TS         int64                      `json:"TS"`
	}
	if err := json.Unmarshal(payloads[len(payloads)-1], &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Change.TYPE != "deleted" || msg.Change.KEY != "CINEMA" || msg.TS <= 0 {
		t.Errorf("CHANGE/TS = %+v %d", msg.Change, msg.TS)
	}
	if strings.Contains(string(msg.Categories), `"CINEMA"`) {
		t.Error("une catégorie supprimée ne doit plus figurer dans CATEGORIES")
	}
	get := dev285Do(s, "GET", "/api/categories", nil, "")
	var a, b interface{}
	_ = json.Unmarshal(msg.Categories, &a)
	_ = json.Unmarshal(get.Body.Bytes(), &b)
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) {
		t.Errorf("CATEGORIES ≠ GET /api/categories\n%s\n%s", aj, bj)
	}
}
