all: build

build:
	@cd havok ; godep restore ; godep go build

container: build
	@cd havok ; docker build -t ehazlett/havok .

.PHONY: all build container
