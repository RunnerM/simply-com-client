package simplyComClient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

var apiUrl = "https://api.simply.com/2"

type RecordType string

var recordTypes = [...]RecordType{
	"A", "AAAA", "AFSDB", "ALIAS", "ANAME", "CAA", "CERT", "CNAME",
	"DNAME", "DNSKEY", "DS", "HINFO", "KEY", "KX", "LOC", "MB",
	"MG", "MINFO", "MR", "MX", "NAPTR", "NS", "PTR", "RP",
	"RRSIG", "SOA", "SPF", "SRV", "SSHFP", "TLSA", "TXT", "URI",
	"WKS", "X25",
}

func validateRecordType(recordType RecordType) bool {
	for _, t := range recordTypes {
		if t == recordType {
			return true
		}
	}
	return false
}

// CreateSimplyClient returns a client authenticated against the Simply.com
// API. apiKey is sent as a Bearer token on every request. accountName is no
// longer used by the API and may be empty; it is kept for compatibility.
//
// Example:
//
//	client := simplyComClient.CreateSimplyClient("", "abcdef...")
//	id, err := client.AddRecord("_acme-challenge.example.com", "token", "TXT")
//	if err != nil {
//	    // handle err
//	}
//	_ = id
//
// See https://www.simply.com/en/docs/api/ for how to obtain an API key.
func CreateSimplyClient(accountName string, apiKey string) SimplyClient {
	return SimplyClient{
		Credentials: Credentials{
			AccountName: accountName,
			ApiKey:      apiKey,
		},
		Logger:     log.New(),
		DefaultTTL: 3600,
		products:   &productCache{},
	}
}

// SimplyClient base type
type SimplyClient struct {
	Credentials Credentials `json:"credentials"`
	Logger      *log.Logger
	DefaultTTL  int

	products *productCache
}

type productCache struct {
	mu      sync.Mutex
	objects map[string]string
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
	AccountName string `json:"-"`
	ApiKey      string `json:"-"`
}

func (c *SimplyClient) doRequest(method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, apiUrl+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request (%s %s): %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}
	req.Header.Set("Authorization", "Bearer "+c.Credentials.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("simply.com API request failed (%s %s): %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body (%s %s): %w", method, path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("simply.com API error (%s %s, HTTP %d): %s",
			method, path, resp.StatusCode, extractAPIError(data))
	}
	return data, nil
}

func extractAPIError(body []byte) string {
	if len(body) == 0 {
		return "no response body"
	}
	var probe struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(body, &probe) == nil {
		if probe.Error != "" {
			return probe.Error
		}
		if probe.Message != "" {
			return probe.Message
		}
	}
	return string(body)
}

// AddRecord Add record to simply
func (c *SimplyClient) AddRecord(FQDNName string, Value string, recordType RecordType) (int, error) {
	if !validateRecordType(recordType) {
		log.Errorln("invalid record type: ", recordType)
		return 0, errors.New("invalid record type")
	}
	object, name, err := c.resolveProduct(FQDNName)
	if err != nil {
		log.Errorln(err)
		return 0, err
	}

	effectiveTTL := c.DefaultTTL
	if effectiveTTL <= 0 {
		effectiveTTL = 3600
	}

	TXTRecordBody := CreateUpdateRecordBody{
		Type:     recordType,
		Name:     name,
		Data:     Value,
		Priority: 1,
		Ttl:      effectiveTTL,
	}
	postBody, err := json.Marshal(TXTRecordBody)
	if err != nil {
		log.Errorln("Failed to marshal request body: ", err)
		return 0, fmt.Errorf("marshal request body: %w", err)
	}

	path := recordsPath(object)
	responseData, err := c.doRequest("POST", path, bytes.NewBuffer(postBody))
	if err != nil {
		log.Errorln(err)
		return 0, err
	}

	var data CreateRecordResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		log.Errorln("Failed to unmarshal body with error: ", err)
		return 0, fmt.Errorf("unmarshal create-record response: %w", err)
	}
	return data.Record.Id, nil
}

// RemoveRecord Remove record from simply
func (c *SimplyClient) RemoveRecord(RecordId int, DnsName string) bool {
	object, _, err := c.resolveProduct(DnsName)
	if err != nil {
		log.Errorln(err)
		return false
	}
	path := recordsPath(object) + strconv.Itoa(RecordId) + "/"
	if _, err := c.doRequest("DELETE", path, nil); err != nil {
		log.Errorln(err)
		return false
	}
	return true
}

// GetRecord Fetch record by FQDNName, RecordData and RecordType,TTL and priority are ignored, returns id of first record found.
func (c *SimplyClient) GetRecord(FQDNName string, RecordData string, recordType RecordType) (int, string, error) {
	object, subdomain, err := c.resolveProduct(FQDNName)
	if err != nil {
		log.Errorln(err)
		return 0, "", err
	}
	responseData, err := getRecords(object, c)
	if err != nil {
		return 0, "", err
	}
	var records RecordResponse
	if err := json.Unmarshal(responseData, &records); err != nil {
		log.Errorln("Failed to unmarshal body with error: ", err)
		return 0, "", fmt.Errorf("unmarshal records response: %w", err)
	}

	for i := 0; i < len(records.Records); i++ {
		if records.Records[i].Data == RecordData && records.Records[i].Type == string(recordType) && records.Records[i].Name == subdomain {
			return records.Records[i].RecordId, records.Records[i].Data, nil
		}
	}
	if RecordData == "" {
		for i := 0; i < len(records.Records); i++ {
			if records.Records[i].Type == string(recordType) && records.Records[i].Name == subdomain {
				return records.Records[i].RecordId, records.Records[i].Data, nil
			}
		}
	}
	log.Errorln("Record not found")
	return 0, "", errors.New("record not found")
}

// GetRecords Fetch records by FQDNName returns id
func (c *SimplyClient) GetRecords(FQDNName string) (string, error) {
	object, _, err := c.resolveProduct(FQDNName)
	if err != nil {
		log.Errorln(err)
		return "", err
	}
	responseData, err := getRecords(object, c)
	if err != nil {
		return "", err
	}
	return string(responseData), nil
}

func getRecords(object string, c *SimplyClient) ([]byte, error) {
	data, err := c.doRequest("GET", recordsPath(object), nil)
	if err != nil {
		log.Errorln(err)
		return nil, err
	}
	return data, nil
}

// UpdateRecord Update record by RecordId, FQDNName, Value and RecordType
func (c *SimplyClient) UpdateRecord(RecordId int, FQDNName string, Value string, recordType RecordType) (bool, error) {
	if !validateRecordType(recordType) {
		log.Errorln("Invalid record type: ", recordType)
		return false, errors.New("invalid record type")
	}
	object, name, err := c.resolveProduct(FQDNName)
	if err != nil {
		log.Errorln(err)
		return false, err
	}

	effectiveTTL := c.DefaultTTL
	if effectiveTTL <= 0 {
		effectiveTTL = 3600
	}

	TXTRecordBody := CreateUpdateRecordBody{
		Type:     recordType,
		Name:     name,
		Data:     Value,
		Priority: 1,
		Ttl:      effectiveTTL,
	}
	putBody, err := json.Marshal(TXTRecordBody)
	if err != nil {
		log.Errorln("Failed to marshal request body: ", err)
		return false, fmt.Errorf("marshal request body: %w", err)
	}

	path := recordsPath(object) + strconv.Itoa(RecordId) + "/"
	if _, err := c.doRequest("PUT", path, bytes.NewBuffer(putBody)); err != nil {
		log.Errorln(err)
		return false, err
	}
	return true, nil
}

// UpsertAddressRecord updates the A or AAAA record for FQDNName to point at
// Ip, creating it if it does not already exist. A vs AAAA is chosen from the
// IP family.
func (c *SimplyClient) UpsertAddressRecord(FQDNName string, Ip string) (bool, error) {
	parsedIP := net.ParseIP(Ip)
	if parsedIP == nil {
		return false, fmt.Errorf("invalid IP address: %q", Ip)
	}

	recordType := RecordType("A")
	if parsedIP.To4() == nil {
		recordType = RecordType("AAAA")
	}

	id, currentData, err := c.GetRecord(FQDNName, "", recordType)
	if err == nil && id != 0 {
		if currentData == Ip {
			return true, nil
		}
		return c.UpdateRecord(id, FQDNName, Ip, recordType)
	}

	if _, addErr := c.AddRecord(FQDNName, Ip, recordType); addErr != nil {
		return false, addErr
	}
	return true, nil
}

func (c *SimplyClient) resolveProduct(FQDNName string) (string, string, error) {
	objects, err := c.productObjects()
	if err != nil {
		return "", "", err
	}

	fqdnName := strings.ToLower(cutTrailingDotIfExist(FQDNName))
	for candidate := fqdnName; candidate != ""; {
		if object, ok := objects[candidate]; ok {
			name := strings.TrimSuffix(strings.TrimSuffix(fqdnName, candidate), ".")
			return object, name, nil
		}
		dot := strings.IndexByte(candidate, '.')
		if dot < 0 {
			break
		}
		candidate = candidate[dot+1:]
	}
	return "", "", fmt.Errorf("no Simply.com product found for %q", FQDNName)
}

func (c *SimplyClient) productObjects() (map[string]string, error) {
	if c.products == nil {
		c.products = &productCache{}
	}
	c.products.mu.Lock()
	defer c.products.mu.Unlock()

	if c.products.objects != nil {
		return c.products.objects, nil
	}

	data, err := c.doRequest("GET", "/my/products/", nil)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	var response struct {
		Products []struct {
			Object string `json:"object"`
			Domain struct {
				Name    string `json:"name"`
				NameIdn string `json:"name_idn"`
			} `json:"domain"`
		} `json:"products"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("unmarshal products response: %w", err)
	}

	objects := make(map[string]string, len(response.Products)*3)
	for _, p := range response.Products {
		objects[strings.ToLower(p.Object)] = p.Object
	}
	for _, p := range response.Products {
		for _, name := range []string{p.Domain.Name, p.Domain.NameIdn} {
			key := strings.ToLower(name)
			if _, exists := objects[key]; name != "" && !exists {
				objects[key] = p.Object
			}
		}
	}
	c.products.objects = objects
	return objects, nil
}

func recordsPath(object string) string {
	return "/my/products/" + url.PathEscape(object) + "/dns/records/"
}

func cutTrailingDotIfExist(FQDNName string) string {
	fqdnName := FQDNName
	if last := len(fqdnName) - 1; last >= 0 && fqdnName[last] == '.' {
		fqdnName = fqdnName[:last]
	}
	return fqdnName
}
