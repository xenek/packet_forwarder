PLATFORM ?= default

SPI_SPEED ?= 8000000
SPIDEV ?= $(firstword $(wildcard /dev/spidev*))
SPI_CS_CHANGE ?= 0

VID ?= 0x0403
PID ?= 0x6014

DEBUG_AUX ?= 0
DEBUG_HAL ?= 0
DEBUG_SPI ?= 0
DEBUG_REG ?= 0
DEBUG_GPS ?= 0
DEBUG_GPIO ?= 0
DEBUG_LBT ?= 0

lora_gateway/libloragw/inc/default.h:
	@echo "Generating default spi header file"
	@echo "#ifndef _DEFAULT__H_" >> $@
	@echo "#define _DEFAULT__H_" >> $@
	@echo "#define DISPLAY_PLATFORM \"Auto-generated default SPI config\"" >> $@
	@echo "#define SPI_SPEED $(SPI_SPEED)" >> $@
	@echo "#define SPI_DEV_PATH \"$(SPIDEV)\"" >> $@
	@echo "#define SPI_CS_CHANGE $(SPI_CS_CHANGE)" >> $@
	@echo "#define VID $(VID)" >> $@
	@echo "#define PID $(PID)" >> $@
	@echo "#endif" >> $@
	@echo "Generated default spi header file"

### transpose library.cfg into a C header file : config.h

LIBLORAGW_VERSION := `cat lora_gateway/VERSION`

lora_gateway/libloragw/inc/config.h: lora_gateway/VERSION lora_gateway/libloragw/library.cfg lora_gateway/libloragw/inc/$(PLATFORM).h
	@echo "*** Checking libloragw library configuration ***"
	@rm -f $@
	#File initialization
	@echo "#ifndef _LORAGW_CONFIGURATION_H" >> $@
	@echo "#define _LORAGW_CONFIGURATION_H" >> $@
	# Release version
	@echo "Release version   : $(LIBLORAGW_VERSION)"
	@echo "	#define LIBLORAGW_VERSION	"\"$(LIBLORAGW_VERSION)\""" >> $@
  # SPI interface
	@echo "SPI interface     : $(CFG_SPI_MSG)"
	@echo "	#define $(CFG_SPI_OPT)	1" >> $@
	# Debug options
	@echo "	#define DEBUG_AUX	$(DEBUG_AUX)" >> $@
	@echo "	#define DEBUG_SPI	$(DEBUG_SPI)" >> $@
	@echo "	#define DEBUG_REG	$(DEBUG_REG)" >> $@
	@echo "	#define DEBUG_HAL	$(DEBUG_HAL)" >> $@
	@echo "	#define DEBUG_GPS	$(DEBUG_GPS)" >> $@
	@echo "	#define DEBUG_GPIO	$(DEBUG_GPIO)" >> $@
	@echo "	#define DEBUG_LBT	$(DEBUG_LBT)" >> $@
  # Platform selection
	@echo "	#include \"$(PLATFORM).h\"" >> $@
	# end of file
	@echo "#endif" >> $@
	@echo "*** Configuration seems ok ***"
