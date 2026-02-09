dev:
	go run main.go


build:
	docker build -t=suconghou/tools:cachelayer .

docker:
	make build && \
	docker images && \
	docker push suconghou/tools:cachelayer