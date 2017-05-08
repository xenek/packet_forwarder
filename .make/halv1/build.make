## HAL depending on the platform
ifeq ($(GOOS),darwin)
	# MAC
	CFG_SPI := mac
	PLATFORM := imst_rpi
else
	CFG_SPI ?= native
	PLATFORM ?= default
endif

# Build the HAL
hal.build: lora_gateway/libloragw/libloragw.a

### library.cfg configuration file processing

ifeq ($(CFG_SPI),native)
  CFG_SPI_MSG := Linux native SPI driver
  CFG_SPI_OPT := CFG_SPI_NATIVE
else ifeq ($(CFG_SPI),ftdi)
  CFG_SPI_MSG := FTDI SPI-over-USB bridge using libmpsse/libftdi/libusb
  CFG_SPI_OPT := CFG_SPI_FTDI
else ifeq ($(CFG_SPI),mac)
  CFG_SPI_MSG := FTDI SPI-over-USB bridge on the MAC using libmpsse/libftdi/libusb
  CFG_SPI_OPT := CFG_SPI_FTDI
else
  $(error No SPI physical layer selected, check lora_gateway/target.cfg file)
endif

lora_gateway/libloragw/libloragw.a:
	CFG_SPI=$(CFG_SPI) PLATFORM=$(PLATFORM) $(MAKE) all -e -C lora_gateway/libloragw

# Clean the HAL
hal.clean:
	$(MAKE) clean -e -C lora_gateway/libloragw
	@rm -f lora_gateway/libloragw/inc/config.h
	@rm -f lora_gateway/libloragw/inc/default.h
