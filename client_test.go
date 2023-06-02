package client

import (
	"fmt"
	"testing"
)

var fixture = SimplyClient{
	Credentials: Credentials{
		AccountName: "",
		ApiKey:      "",
	},
}

type testData struct {
	domain      string
	data        string
	data2       string
	accountname string
	apikey      string
	basedomain  string
}

// Plot in your own api details for testing.
func TestAll(t *testing.T) {
	data := testData{ //add your credentials here to test.
		domain:      "_acme-challenge.foo.com",
		data:        "test_txt_data",
		data2:       "test_txt_data_2",
		accountname: "",
		apikey:      "",
	}
	testAdd(t, data)
	id := testGet(t, data)
	testUpdate(t, data, id)
	testRemove(t, data, id)

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
	res2, _ := fixture.GetExactTxtRecord(data.data2, data.domain)

	if res2 != id {
		t.Fail()
	}

	res := fixture.RemoveRecord(id, data.domain)
	if res != true {
		t.Fail()
	}

}
func testGet(t *testing.T, data testData) int {
	id, recData, _ := fixture.GetRecord(data.domain)
	if id == 0 {
		t.Fail()
	}
	if recData == "" {
		t.Fail()
	}
	return id
}
