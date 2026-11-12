package recipe

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestDocumentRoundTrip(t *testing.T) {
	d := Document{Version: DocumentVersion, Name: "Хлеб", Items: []Ingredient{{Name: "Мука", Mass: 2.5, Moisture: 14}}, Params: map[string]float64{"Wt": 47.5}, Groups: map[string]bool{"Сухарики": true}}
	var buf bytes.Buffer
	if err := WriteDocument(&buf, d); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDocument(&buf)
	if err != nil || !reflect.DeepEqual(got, d) {
		t.Fatalf("получено %#v, ошибка %v", got, err)
	}
}

func TestDocumentRejectsInvalid(t *testing.T) {
	for _, data := range []string{
		`{}`,
		`{"version":2,"name":"x","ingredients":[],"parameters":{},"groups":{}}`,
		`{"version":1,"name":"x","ingredients":[{"Name":"","Mass":1,"Moisture":0}],"parameters":{},"groups":{}}`,
		`{"version":1,"name":"x","ingredients":[],"parameters":{},"groups":{},"extra":0}`,
		`{"version":1,"name":"x","ingredients":[],"parameters":{},"groups":{}} {}`,
	} {
		if _, err := ReadDocument(strings.NewReader(data)); err == nil {
			t.Errorf("приняты некорректные данные: %s", data)
		}
	}
}
