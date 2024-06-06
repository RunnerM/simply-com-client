package simplyComClient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/bobesa/go-domain-util/domainutil"
	log "github.com/sirupsen/logrus"
)

const (
	apiUrl = "https://api.simply.com/2"
)

type RecordType string

var recordTypes = [...]RecordType{"A", "AAAA", "CNAME", "MX", "NS", "TXT", "SRV", "CAA", "SSHFP", "TLSA", "PTR", "NAPTR", "DS", "DNSKEY", "RRSIG", "SPF", "URI", "ALIAS", "ANAME", "LOC", "HINFO", "RP", "AFSDB", "CERT", "DNAME", "KEY", "KX", "MB", "MG", "MINFO", "MR", "MX", "NAPTR", "NS", "PTR", "SOA", "TXT", "WKS", "X25"}

func validateRecordType(recordType RecordType) bool {
	for _, t := range recordTypes {
		if t == recordType {
			return true
		}
	}
	return false
}

func CreateSimplyClient(accountName string, apiKey string) SimplyClient {
	return SimplyClient{
		Credentials{
			AccountName: accountName,
			ApiKey:      apiKey,
		},
		log.New(),
	}
}

// SimplyClient base type
type SimplyClient struct {
	Credentials Credentials `json:"credentials"`
	Logger      *log.Logger
}

// RecordResponse api type
type RecordResponse struct {
	Records []struct {
		RecordId int    `json:"record_id"`
		Name     string `json:"name"`
		Ttl      int    `json:"ttl"`
		Data     string `json:"data"`
		Type     string `json:"type"`
		Priority int    `json:"priority"`
	} `json:"records"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// CreateUpdateRecordBody api type
type CreateUpdateRecordBody struct {
	Type     RecordType `json:"type"`
	Name     string     `json:"name"`
	Data     string     `json:"data"`
	Priority int        `json:"priority"`
	Ttl      int        `json:"ttl"`
}

// CreateRecordResponse api type
type CreateRecordResponse struct {
	Record struct {
		Id int `json:"id"`
	} `json:"record"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type Credentials struct {
	AccountName string `json:"status"`
	ApiKey      string `json:"message"`
}

// AddRecord Add record to simply
func (c *SimplyClient) AddRecord(FQDNName string, Value string, recordType RecordType) (int, error) {
	if !validateRecordType(recordType) {
		log.Errorln("invalid record type: ", recordType)
		return 0, errors.New("invalid record type")
	}
	// Trim one trailing dot
	fqdnName := cutTrailingDotIfExist(FQDNName)
	TXTRecordBody := CreateUpdateRecordBody{
		Type:     recordType,
		Name:     domainutil.Subdomain(fqdnName),
		Data:     Value,
		Priority: 1,
		Ttl:      3600,
	}
	postBody, _ := json.Marshal(TXTRecordBody)
	req, err := http.NewRequest("POST", apiUrl+"/my/products/"+domainutil.Domain(fqdnName)+"/dns/records", bytes.NewBuffer(postBody))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.SetBasicAuth(c.Credentials.AccountName, c.Credentials.ApiKey)
	client := &http.Client{}
	response, err := client.Do(req)

	if err != nil || response.StatusCode != 200 {
		log.Errorln("Failed request, response: ", response.StatusCode)
		return 0, err
	}
	responseData, err := io.ReadAll(response.Body)

	if err != nil {
		log.Errorln("Failed to read body with error: ", err)
		return 0, err
	}
	var data CreateRecordResponse

	err = json.Unmarshal(responseData, &data)
	if err != nil {
		log.Errorln("Failed to unmarshal body with error: ", err)
		return 0, err
	}
	return data.Record.Id, nil
}

// RemoveRecord Remove record from simply
func (c *SimplyClient) RemoveRecord(RecordId int, DnsName string) bool {
	dnsName := cutTrailingDotIfExist(DnsName)
	req, err := http.NewRequest("DELETE", apiUrl+"/my/products/"+domainutil.Domain(dnsName)+"/dns/records/"+strconv.Itoa(RecordId), nil)
	req.SetBasicAuth(c.Credentials.AccountName, c.Credentials.ApiKey)
	client := &http.Client{}
	response, err := client.Do(req)

	if err != nil || response.StatusCode != 200 {
		log.Errorln("Failed request, response code: ", response.StatusCode)
		return false
	}
	return true
}

// GetRecord Fetch record by FQDNName, RecordData and RecordType,TTL and priority are ignored, returns id of first record found.
func (c *SimplyClient) GetRecord(FQDNName string, RecordData string, recordType RecordType) (int, string, error) {
	fqdnName := cutTrailingDotIfExist(FQDNName)
	responseData, err2, failed := getRecords(fqdnName, c)
	if failed {
		return 0, "", err2
	}
	var records RecordResponse
	err := json.Unmarshal(responseData, &records)

	if err == nil {
		for i := 0; i < len(records.Records); i++ {
			if records.Records[i].Data == RecordData && records.Records[i].Type == string(recordType) && records.Records[i].Name == domainutil.Subdomain(fqdnName) {
				return records.Records[i].RecordId, records.Records[i].Data, nil
			}
		}
		if RecordData == "" {
			for i := 0; i < len(records.Records); i++ {
				if records.Records[i].Type == string(recordType) && records.Records[i].Name == domainutil.Subdomain(fqdnName) {
					return records.Records[i].RecordId, records.Records[i].Data, nil
				}
			}
		}
		log.Errorln("Record not found")
		return 0, "", errors.New("record not found")
	} else {
		log.Errorln("Failed to unmarshal body with error: ", err)
		return 0, "", err
	}
}

// GetRecords Fetch records by FQDNName returns id
func (c *SimplyClient) GetRecords(FQDNName string) (string, error) {
	fqdnName := cutTrailingDotIfExist(FQDNName)
	responseData, err2, failed := getRecords(fqdnName, c)
	if failed {
		return "", err2
	}
	return string(responseData), nil
}

func getRecords(fqdnName string, c *SimplyClient) ([]byte, error, bool) {
	req, err := http.NewRequest("GET", apiUrl+"/my/products/"+domainutil.Domain(fqdnName)+"/dns/records", nil)
	req.SetBasicAuth(c.Credentials.AccountName, c.Credentials.ApiKey)
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil || response.StatusCode != 200 {
		log.Errorln("Failed request, response code: ", response.StatusCode)
		return nil, err, true
	}
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		log.Errorln("Error on read: ", err)
		return nil, err, true
	}
	return responseData, nil, false
}

// UpdateRecord Update record by RecordId, FQDNName, Value and RecordType
func (c *SimplyClient) UpdateRecord(RecordId int, FQDNName string, Value string, recordType RecordType) (bool, error) {
	if !validateRecordType(recordType) {
		log.Errorln("Invalid record type: ", recordType)
		return false, errors.New("invalid record type")
	}
	// Trim one trailing dot
	fqdnName := cutTrailingDotIfExist(FQDNName)
	TXTRecordBody := CreateUpdateRecordBody{
		Type:     recordType,
		Name:     domainutil.Subdomain(fqdnName),
		Data:     Value,
		Priority: 1,
		Ttl:      3600,
	}
	putBody, _ := json.Marshal(TXTRecordBody)
	req, err := http.NewRequest("PUT", apiUrl+"/my/products/"+domainutil.Domain(fqdnName)+"/dns/records/"+strconv.Itoa(RecordId), bytes.NewBuffer(putBody))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.SetBasicAuth(c.Credentials.AccountName, c.Credentials.ApiKey)
	client := &http.Client{}
	response, err := client.Do(req)

	if err != nil || response.StatusCode != 200 {
		log.Errorln("Failed request, response code: ", response.StatusCode)
		return false, err
	}
	return true, nil
}

// DDNS update/create record if Ip omited Simply api will use client IP
func (c *SimplyClient) UpdateDDNS(FQDNName string, Ip string) (bool, error) {
	fqdnName := cutTrailingDotIfExist(FQDNName)
	domain := domainutil.Domain(fqdnName)
	path := ""
	if Ip == "" {
		path = "/ddns/?domain="+ domain+ "&hostname=" + fqdnName
	} else {
		path = "/ddns/?domain="+ domain+ "&hostname=" + fqdnName + "&myip=" + Ip
	}
	//body, _ := json.Marshal("")
	req, err := http.NewRequest("POST", apiUrl+path, nil)
	req.SetBasicAuth(c.Credentials.AccountName, c.Credentials.ApiKey)
	client := &http.Client{}
	response, err := client.Do(req)

	if err != nil || response.StatusCode != 200 {
		log.Errorln("Failed request, response code: ", response.StatusCode)
		return false, err
	}
	return true, nil
}

func cutTrailingDotIfExist(FQDNName string) string {
	fqdnName := FQDNName
	if last := len(fqdnName) - 1; last >= 0 && fqdnName[last] == '.' {
		fqdnName = fqdnName[:last]
	}
	return fqdnName
}
