include .env
export

SHELL := /bin/bash

.PHONY: build run-test-v1 run-test-v2 run-test-v3 run-test-v4 run-test-v5 run-test-v6 run-test-v7 run-test-v8 \
	run-test-v9 run-test-v10 run-test-v11 run-test-v12 run-test-v13 run-test-v14 run-test-v14-race clean \
	db-status db-up db-down db-reset db-create \
	env-up env-down


LOCAL_DB_DSN ?= postgres://postgres:postgres@localhost:5432/praktikum?sslmode=disable

DOCKER_DB_DSN ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable

build:
	@echo "Компиляция сервера и агента в папку bin/..."
	@mkdir -p bin
	go build -o ./bin/server ./cmd/server
	go build -o ./bin/agent ./cmd/agent

run-test-v1: build
	@echo "Запуск автотестов для Iteration 1..."
	@chmod +x ./metricstest_v2
	./metricstest_v2 -test.v -test.run=^TestIteration1$$ \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server

run-test-v2: build
	@echo "Запуск автотестов для Iteration 2..."
	@chmod +x ./metricstest_v2
	./metricstest_v2 -test.v -test.run="^TestIteration2[AB]*$$" \
		-source-path=. \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server
		
run-test-v3: build
	@echo "Запуск автотестов для Iteration 3..."
	@chmod +x ./metricstest_v2
	./metricstest_v2 -test.v -test.run="^TestIteration3[AB]*$$" \
		-source-path=. \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server		

run-test-v4: build
	@echo "Запуск автотестов для Iteration 4..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	./metricstest_v2 -test.v -test.run="^TestIteration4$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-server-port=$$SERVER_PORT \
		-source-path=.	
			
run-test-v5: build
	@echo "Запуск автотестов для Iteration 5..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration5$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE

run-test-v6: build
	@echo "Запуск автотестов для Iteration 6..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration6$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE

run-test-v7: build
	@echo "Запуск автотестов для Iteration 7..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	./metricstest_v2 -test.v -test.run="^TestIteration7$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-server-port=$$SERVER_PORT \
		-source-path=.

run-test-v8: build
	@echo "Запуск автотестов для Iteration 8..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	./metricstest_v2 -test.v -test.run="^TestIteration8$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-server-port=$$SERVER_PORT \
		-source-path=.

run-test-v9: build
	@echo "Запуск автотестов для Iteration 9..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration9$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-file-storage-path=$$TEMP_FILE \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE	

run-test-v10: build
	@echo "Запуск автотестов для Iteration 10..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration10[AB]$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-database-dsn='$(LOCAL_DB_DSN)' \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE

run-test-v11: build
	@echo "Запуск автотестов для Iteration 11..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration11$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-database-dsn='$(LOCAL_DB_DSN)' \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE

run-test-v12: build
	@echo "Запуск автотестов для Iteration 12..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration12$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-database-dsn='$(LOCAL_DB_DSN)' \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE

run-test-v13: build
	@echo "Запуск автотестов для Iteration 13..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration13$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-database-dsn='$(LOCAL_DB_DSN)' \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE

run-test-v14: build
	@echo "Запуск автотестов для Iteration 14..."
	@chmod +x ./metricstest_v2
	@SERVER_PORT=$$(( (RANDOM % 10000) + 20000 )); \
	ADDRESS="localhost:$$SERVER_PORT"; \
	TEMP_FILE=$$(mktemp); \
	ADDRESS=$$ADDRESS TEMP_FILE=$$TEMP_FILE ./metricstest_v2 -test.v -test.run="^TestIteration14$$" \
		-agent-binary-path=./bin/agent \
		-binary-path=./bin/server \
		-database-dsn='$(LOCAL_DB_DSN)' \
		-key="$$TEMP_FILE" \
		-server-port=$$SERVER_PORT \
		-source-path=.; \
	rm -f $$TEMP_FILE

run-test-v14-race:
	@echo "Запуск проверки на состояние гонки (race detector)..."
	go test -v -race ./...		

clean:
	@echo "Очистка скомпилированных файлов..."
	rm -rf bin

db-status:
	docker compose run --rm migrations postgres "$(DOCKER_DB_DSN)" status

db-up:
	docker compose run --rm migrations postgres "$(DOCKER_DB_DSN)" up

db-down:
	docker compose run --rm migrations postgres "$(DOCKER_DB_DSN)" down

db-reset:
	docker compose run --rm migrations postgres "$(DOCKER_DB_DSN)" reset

db-create:
	docker compose run --rm -v .:/app -w /app/postgres/migrations migrations create $(NAME) sql	

run:
	go run cmd/server/main.go -d "$(LOCAL_DB_DSN)"

env-up:
	docker compose up -d postgres
	docker compose up migrations

env-down:
	docker compose down		