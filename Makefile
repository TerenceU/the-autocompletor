BINARY = theautocompletor
INSTALL_PATH = /usr/local/bin

.PHONY: build install uninstall clean

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

uninstall:
	@echo "Removing $(BINARY)..."
	rm -f $(INSTALL_PATH)/$(BINARY)
	@# Remove tac alias only if it points to our binary
	@if [ -L $(INSTALL_PATH)/tac ] && [ "$$(readlink $(INSTALL_PATH)/tac)" = "$(INSTALL_PATH)/$(BINARY)" ]; then \
		rm -f $(INSTALL_PATH)/tac; \
		echo "Alias 'tac' removed."; \
	fi
	@echo "Done."

clean:
	rm -f $(BINARY)
