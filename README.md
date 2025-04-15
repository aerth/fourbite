4byte utilities

## 4server: http api server

serves from db created with 4index

eg: 

`curl http://localhost:8081/4byte/06fdde03,23b872dd,313ce567`

```json
["name()","transferFrom(address,address,uint256)","decimals()"]
```

`curl http://localhost:8081/4byte/313ce567`

```json
["decimals()"]
```

## 4byte: cli tool

uses db created with 4index

## 4index: create optimized db

uses [4bytes](https://github.com/ethereum-lists/4bytes/tree/master/signatures) repo to create a 100MB db file

this db is used (readonly) by `4byte` and `4server` to quickly lookup function signatures

new: we now read from the zip file directly (much faster)

