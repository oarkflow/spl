VSCODE_EXTENSION_DIR := vscode-extension
VSCODE_EXTENSION_ID := oarkflow.oarkflow-template-vscode
VSCODE_EXTENSION_INSTALL_DIR := $(HOME)/.vscode/extensions/$(VSCODE_EXTENSION_ID)
VSCODE_CLI ?= code

.PHONY: vscode-extension-check install-extension uninstall-old-spl-extension reload-vscode vscode-extension features

vscode-extension-check:
	cd $(VSCODE_EXTENSION_DIR) && npm run check

install-extension: vscode-extension-check
	mkdir -p "$(HOME)/.vscode/extensions"
	rm -rf "$(VSCODE_EXTENSION_INSTALL_DIR)"
	cp -R "$(VSCODE_EXTENSION_DIR)" "$(VSCODE_EXTENSION_INSTALL_DIR)"
	@echo "Installed $(VSCODE_EXTENSION_ID) to $(VSCODE_EXTENSION_INSTALL_DIR)"

uninstall-old-spl-extension:
	rm -rf "$(HOME)/.vscode/extensions/oarkflow.spl-vscode-0.1.0"
	@echo "Removed old oarkflow.spl-vscode extension if it existed"

reload-vscode:
	@$(VSCODE_CLI) --reuse-window .
	@echo "VS Code CLI on this machine does not expose a reliable reload command."
	@echo "Use Command Palette: Developer: Reload Window"

features:
	go run ./cmd/features

vscode-extension: uninstall-old-spl-extension install-extension reload-vscode
