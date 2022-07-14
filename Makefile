gen:
	protoc --proto_path=proto proto/*.proto --go_out=./pb --go_opt=paths=source_relative --go-grpc_out=./pb --go-grpc_opt=require_unimplemented_servers=false,paths=source_relative

clean:
	rm pb/pcbook/*.go

server:
	go run cmd/server/main.go -port 8080

client:
	go run cmd/client/main.go -address localhost:8080

test:
	go test -cover -race ./...

cert:
	cd cert; sh ./gen.sh; cd ..

.PHONY: gen clean server client test cert