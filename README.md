<div align="center">

# A partial client library for simply.com dns provider's API.

![GitHub](https://img.shields.io/github/license/runnerm/simply-com-client) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/runnerm/simply-com-client) 
	[![Go Report Card](https://goreportcard.com/badge/github.com/runnerm/simply-com-client)](https://goreportcard.com/report/github.com/runnerm/simply-com-client)
</div>

This is partial implementation of simply.com dns provider's API. Any contribution is welcome in the 
form of PR's. Further documentation of the API can be found [here](https://www.simply.com/dk/docs/api/).

## Usage 
Add this repository as go dependency.

``` go
import (
	"github.com/runnerm/simply-com-client"
)
```
Create a new client with your API key.
``` go
client := CreateSimplyClient("accountName", "apiKey")
```
Use the client to interact with the API.
``` go
// Get record for a domain
records, err := client.GetRecord("example.com")
```

## Implemented methods
- [x] GetRecord
- [x] AddRecord
- [x] RemoveRecord
- [x] UpdateRecord
- [x] UpdateDDNS

