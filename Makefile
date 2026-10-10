-include .env
export

run:
	go run .

test:
	go test -race -count=1 ./...

newkey:
	go run . newkey $(NAME)
