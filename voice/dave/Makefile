# Builds libdave static library and installs headers + .a to libdave/cpp/build/install.
# The install step merges libdave.a and all vcpkg dependency archives into a single
# combined static library so CGo only needs to link one file.
LIBDAVE_CPP = libdave/cpp
BUILD_DIR   = $(LIBDAVE_CPP)/build
INSTALL_DIR = $(BUILD_DIR)/install
VCPKG_DIR   = $(LIBDAVE_CPP)/vcpkg

# vcpkg installs into a triplet-named subdirectory; located at rule-execution time.
VCPKG_INSTALLED = $(CURDIR)/$(BUILD_DIR)/vcpkg_installed
ABS_INSTALL_DIR = $(CURDIR)/$(INSTALL_DIR)

.PHONY: all clean

all: $(INSTALL_DIR)/lib/libdave.a

$(INSTALL_DIR)/lib/libdave.a: $(LIBDAVE_CPP)/CMakeLists.txt
	@if [ ! -f "$(VCPKG_DIR)/vcpkg" ] && [ ! -f "$(VCPKG_DIR)/vcpkg.exe" ]; then \
		echo "Bootstrapping vcpkg..."; \
		if [ ! -f "$(VCPKG_DIR)/bootstrap-vcpkg.sh" ]; then \
			cd $(LIBDAVE_CPP) && git submodule update --init vcpkg; \
		fi; \
		cd $(CURDIR)/$(VCPKG_DIR) && ./bootstrap-vcpkg.sh -disableMetrics; \
	fi
	$(MAKE) -C $(LIBDAVE_CPP) all install
	@echo "Merging vcpkg dependency archives into libdave.a..."
	@TRIPLET=$$(ls $(VCPKG_INSTALLED) | grep -v '^vcpkg$$' | head -1); \
	VCPKG_LIB=$(VCPKG_INSTALLED)/$$TRIPLET/lib; \
	mkdir -p $(ABS_INSTALL_DIR)/lib/merge_tmp && \
	cd $(ABS_INSTALL_DIR)/lib/merge_tmp && \
		mkdir dave && (cd dave && ar x $(ABS_INSTALL_DIR)/lib/libdave.a) && \
		EXTRA=""; \
		if [ -f /usr/lib/libstdc++.a ]; then EXTRA=/usr/lib/libstdc++.a; fi; \
		for lib in $$VCPKG_LIB/libmlspp.a $$VCPKG_LIB/libhpke.a \
		            $$VCPKG_LIB/libtls_syntax.a $$VCPKG_LIB/libbytes.a \
		            $$VCPKG_LIB/libssl.a $$VCPKG_LIB/libcrypto.a \
		            $$EXTRA; do \
			name=$$(basename $$lib .a) && mkdir $$name && (cd $$name && ar x $$lib); \
		done && \
		ar rcs $(ABS_INSTALL_DIR)/lib/libdave.a */*.o
	@rm -rf $(ABS_INSTALL_DIR)/lib/merge_tmp
	@echo "Done: $(INSTALL_DIR)/lib/libdave.a"

clean:
	$(MAKE) -C $(LIBDAVE_CPP) cclean
