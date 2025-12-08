.PHONY: proto clean build run

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/*.proto

clean:
	rm -f api/proto/*.pb.go
	docker-compose down -v

build:
	docker-compose build

run:
	docker-compose up

dev:
	docker-compose up --build

stop:
	docker-compose down

logs:
	docker-compose logs -f
