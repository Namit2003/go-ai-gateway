-include .env
export

run:
	go run .

test:
	go test -race -count=1 ./...
