dev:
	go run main.go


build:
	docker build -t=registry.cn-beijing.aliyuncs.com/suconghou/tools:gateway .

docker:
	make build && \
	docker images && \
	docker push registry.cn-beijing.aliyuncs.com/suconghou/tools:gateway