all:
	@cd havok ; godep restore ; godep go build

.PHONY: all
