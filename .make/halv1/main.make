## HAL depending on the platform
ifeq ($(GOOS),darwin)
	# MAC
	CFG_SPI := mac
	PLATFORM := imst_rpi
endif

HAL_REPO := https://github.com/TheThingsNetwork/lora_gateway.git

## install dependencies
hal.deps:
	@$(log) "fetching HAL and dependencies"
	git clone -b master $(HAL_REPO) ./lora_gateway

## clean dependencies
hal.clean-deps:
	@$(log) "cleaning HAL" [rm -rf lora_gateway/]
	@rm -rf lora_gateway/
