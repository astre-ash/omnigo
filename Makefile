.PHONY: build run-test clean

build:
	@echo "Компиляция сервера и агента в папку bin/..."
	@mkdir -p bin
	go build -o ./bin/server ./cmd/server/*.go
	go build -o ./bin/agent ./cmd/agent/*.go

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

clean:
	@echo "Очистка скомпилированных файлов..."
	rm -rf bin
