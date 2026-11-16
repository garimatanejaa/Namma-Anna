.PHONY: dev-api dev-web build run train test data docker
dev-api:   ## Go API on :8080
	cd backend && go run ./cmd/server
dev-web:   ## Vite dev server on :5173 (proxies /api)
	cd web && npm install && npm run dev
build:     ## single binary with the UI embedded -> backend/riderl
	cd web && npm install && npm run build
	cd backend && go build -o riderl ./cmd/server
run: build
	cd backend && ./riderl
train:     ## re-train policies and refresh the benchmark (~10 min)
	cd backend && go run ./cmd/train -es 40
test:
	cd backend && go test ./...
data:      ## rebuild ward model from data/raw (needs python3, shapely, numpy)
	python3 data/pipeline/build_city.py
docker:
	docker build -t riderl-bengaluru .
