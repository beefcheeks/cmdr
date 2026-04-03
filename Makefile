BIN_DIR     := $(HOME)/.local/bin
PLIST_NAME  := com.mikehu.cmdr.plist
LAUNCH_DIR  := $(HOME)/Library/LaunchAgents
GUI_DOMAIN  := gui/$(shell id -u)

.PHONY: all build web go install uninstall restart clean dev

# Default: build everything
all: build

# Build frontend + backend
build: web go

# Build SvelteKit SPA → web/build/
web:
	@echo "cmdr: building frontend..."
	@cd web && bun run build

# Build Go binary (embeds web/build/)
go:
	@echo "cmdr: building backend..."
	@go build -o cmdr ./cmd/cmdr

# Full deploy: build → install binary → restart service
install: build
	@mkdir -p $(BIN_DIR) $(LAUNCH_DIR)
	@cp cmdr $(BIN_DIR)/cmdr
	@echo "cmdr: installed binary to $(BIN_DIR)/cmdr"
	@launchctl bootout "$(GUI_DOMAIN)/$(PLIST_NAME)" 2>/dev/null || true
	@sed 's|__CMDR_BIN__|$(BIN_DIR)/cmdr|g' $(PLIST_NAME) > $(LAUNCH_DIR)/$(PLIST_NAME)
	@launchctl bootstrap "$(GUI_DOMAIN)" "$(LAUNCH_DIR)/$(PLIST_NAME)"
	@rm -f cmdr
	@echo "cmdr: service installed and started ✓"

# Stop and remove service
uninstall:
	@launchctl bootout "$(GUI_DOMAIN)/$(PLIST_NAME)" 2>/dev/null || true
	@rm -f $(BIN_DIR)/cmdr $(LAUNCH_DIR)/$(PLIST_NAME)
	@echo "cmdr: uninstalled ✓"

# Restart service without rebuilding
restart:
	@launchctl bootout "$(GUI_DOMAIN)/$(PLIST_NAME)" 2>/dev/null || true
	@launchctl bootstrap "$(GUI_DOMAIN)" "$(LAUNCH_DIR)/$(PLIST_NAME)"
	@echo "cmdr: restarted ✓"

# Dev: just Vite HMR, proxies API to production daemon
dev:
	@cd web && bun run vite dev

# Run Go tests
test:
	@go test ./...

# Clean build artifacts
clean:
	@rm -f cmdr
	@rm -rf web/build
	@echo "cmdr: cleaned ✓"
