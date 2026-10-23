package simplyComClient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Plot in your own api details for testing.
var fixture = CreateSimplyClient("", "")

type testData struct {
	domain  string
	data    string
	data2   string
	bogusIP string
}

func TestAll(t *testing.T) {
	if fixture.Credentials.ApiKey == "" {
		t.Skip("no API key set in fixture")
	}
	data := testData{
		domain:  "test2.ddns.example.dk",
		data:    "test_txt_data",
		data2:   "test_txt_data_2",
		bogusIP: "19.19.19.20",
	}
	testAdd(t, data)
	id := testGet(t, data)
	testUpdate(t, data, id)
	testRemove(t, data, id)
	testUpsertAddressRecord(t, data)

}

func testAdd(t *testing.T, data testData) {
	id, err := fixture.AddRecord(data.domain, data.data, "TXT")
	if err != nil {
		t.Fail()
	}
	if id == 0 {
		t.Fail()
	}
	fmt.Println(id)
}

func testUpdate(t *testing.T, data testData, id int) {
	res, err := fixture.UpdateRecord(id, data.domain, data.data2, "TXT")
	if err != nil {
		t.Fail()
	}
	if res != true {
		t.Fail()
	}
	fmt.Println(id)
}

func testRemove(t *testing.T, data testData, id int) {
	res2, _, _ := fixture.GetRecord(data.domain, data.data2, "TXT")

	if res2 != id {
		t.Fail()
	}

	res := fixture.RemoveRecord(id, data.domain)
	if res != true {
		t.Fail()
	}

}
func testGet(t *testing.T, data testData) int {
	id, recData, _ := fixture.GetRecord(data.domain, "", "TXT")
	if id == 0 {
		t.Fail()
	}
	if recData == "" {
		t.Fail()
	}
	return id
}

func testUpsertAddressRecord(t *testing.T, data testData) {
	res, err := fixture.UpsertAddressRecord(data.domain, data.bogusIP)
	if err != nil {
		t.Fail()
	}
	if res != true {
		t.Fail()
	}
}

func TestProductLookup(t *testing.T) {
	var requests []string
	productLists := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected Authorization header %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path == "/my/products/" {
			productLists++
			writeJSON(w, map[string]any{"products": []map[string]any{
				{"object": "kvolbæk.dk", "domain": map[string]any{"name": "kvolbæk.dk", "name_idn": "xn--kvolbk-tua.dk"}},
				{"object": "example.com", "domain": map[string]any{"name": "example.com", "name_idn": "example.com"}},
				{"object": "www.example.com", "domain": map[string]any{"name": "www.example.com", "name_idn": "www.example.com"}},
			}})
			return
		}
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.Path+" "+string(body))
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]any{"records": []map[string]any{
				{"record_id": 7, "name": "_acme-challenge", "type": "TXT", "data": "token"},
			}})
		case http.MethodPost:
			writeJSON(w, map[string]any{"record": map[string]any{"id": 42}})
		default:
			writeJSON(w, map[string]any{"status": 200})
		}
	}))
	defer server.Close()

	originalURL := apiUrl
	apiUrl = server.URL
	defer func() { apiUrl = originalURL }()

	client := CreateSimplyClient("", "secret")

	id, err := client.AddRecord("_acme-challenge.XN--KVOLBK-TUA.DK.", "token", "TXT")
	if err != nil || id != 42 {
		t.Fatalf("AddRecord = %d, %v", id, err)
	}
	id, data, err := client.GetRecord("_acme-challenge.kvolbæk.dk", "", "TXT")
	if err != nil || id != 7 || data != "token" {
		t.Fatalf("GetRecord = %d, %q, %v", id, data, err)
	}
	if _, err := client.UpdateRecord(7, "_acme-challenge.www.example.com", "token2", "TXT"); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	if !client.RemoveRecord(7, "_acme-challenge.example.com") {
		t.Fatal("RemoveRecord failed")
	}

	expected := []string{
		`POST /my/products/kvolbæk.dk/dns/records/ {"type":"TXT","name":"_acme-challenge","data":"token","priority":1,"ttl":3600}`,
		`GET /my/products/kvolbæk.dk/dns/records/ `,
		`PUT /my/products/www.example.com/dns/records/7/ {"type":"TXT","name":"_acme-challenge","data":"token2","priority":1,"ttl":3600}`,
		`DELETE /my/products/example.com/dns/records/7/ `,
	}
	if len(requests) != len(expected) {
		t.Fatalf("unexpected requests %q", requests)
	}
	for i := range expected {
		if requests[i] != expected[i] {
			t.Errorf("request %d = %q, want %q", i, requests[i], expected[i])
		}
	}
	if productLists != 1 {
		t.Errorf("expected products to be listed once, got %d", productLists)
	}

	if _, err := client.AddRecord("_acme-challenge.unknown.dk", "token", "TXT"); err == nil {
		t.Error("expected error for a domain without a product")
	}
	if client.RemoveRecord(7, "unknown.dk") {
		t.Error("expected RemoveRecord to fail for a domain without a product")
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(err)
	}
}
