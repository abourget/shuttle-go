UUID := shuttle-pro@shuttle-go.dev
VERSION := $(shell jq -r '.version' metadata.json)
TARBALL := $(UUID)-v$(VERSION).tar.gz

.PHONY: all clean install enable

all: $(TARBALL)

$(TARBALL): extension.js metadata.json
	tar -czf $@ extension.js metadata.json

# Install into the current user's GNOME extensions directory.
install: $(TARBALL)
	gnome-extensions install $(TARBALL)

# Enable the installed extension.
enable:
	gnome-extensions enable $(UUID)

clean:
	rm -f $(TARBALL)
