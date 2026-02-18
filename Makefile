BINARY = theautocompleter
INSTALL_PATH = /usr/local/bin

.PHONY: build install clean

build:
	go build -o $(BINARY) .

install: build
	@echo "Installing $(BINARY) to $(INSTALL_PATH)..."
	install -m 755 $(BINARY) $(INSTALL_PATH)/$(BINARY)
	@# Install tac alias only if not already taken by system
	@if ! command -v tac >/dev/null 2>&1; then \
		ln -sf $(INSTALL_PATH)/$(BINARY) $(INSTALL_PATH)/tac; \
		echo "Alias 'tac' installed."; \
	else \
		echo "Skipping 'tac' alias (already used by system)."; \
	fi

clean:
	rm -f $(BINARY)
