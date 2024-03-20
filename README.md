#  RequestBox

a server that stores recent requests

# build

    $docker build -t requestbox .  

## run with docker

    $docker run -p 127.0.0.1:8080:8080/tcp requestbox

## run local

    $go run main.go

## create token

    $curl -X POST http://localhost:8080/CreateToken

## list token

    $curl -X POST http://localhost:8080/ListToken

## Send request to requestbox token example

    $curl -X POST http://localhost:8080/PostRequest?token=687ba0ac-141e-4e7f-a401-d5b728ce22bd -d '{"hi":"here"}'


## Get request from requestbox

    $curl -X POST http://localhost:8080/GetRequest?token=687ba0ac-141e-4e7f-a401-d5b728ce22bd

## delete token

    $curl -X POST http://localhost:8080/DeleteToken?token=687ba0ac-141e-4e7f-a401-d5b728ce22bd




