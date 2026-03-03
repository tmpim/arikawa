# Builds libdave static library and installs headers + .a to libdave/cpp/build/install
LIBDAVE_CPP = libdave/cpp
BUILD_DIR   = $(LIBDAVE_CPP)/build
INSTALL_DIR = $(BUILD_DIR)/install
VCPKG_DIR   = $(LIBDAVE_CPP)/vcpkg

.PHONY: all clean

all: $(INSTALL_DIR)/lib/libdave.a

$(INSTALL_DIR)/lib/libdave.a: $(LIBDAVE_CPP)/CMakeLists.txt
	@if [ ! -f "$(VCPKG_DIR)/vcpkg" ] && [ ! -f "$(VCPKG_DIR)/vcpkg.exe" ]; then \
		echo "Bootstrapping vcpkg..."; \
		cd $(LIBDAVE_CPP) && git submodule update --init vcpkg && cd vcpkg && ./bootstrap-vcpkg.sh -disableMetrics; \
	fi
	$(MAKE) -C $(LIBDAVE_CPP) install

clean:
	$(MAKE) -C $(LIBDAVE_CPP) cclean
