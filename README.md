# LINE for Omarchy

A work-in-progress QML and Go plugin for using LINE chats, replies, attachments, and stickers in Omarchy 4 Quattro. This is not an official LINE product.

## Installation

After preparing the prerequisites, run:

```bash
git clone https://github.com/komagata/line.git && cd line && ./scripts/setup
```

- Linux, Omarchy 4 Quattro / Quickshell, Qt Quick Controls and Dialogs
- For fetching and building the source: git, Go 1.26 or later, Python 3
- For storing authentication: libsecret's `/usr/bin/secret-tool` and an unlocked Secret Service
- For notifications: `/usr/bin/notify-send` (on Arch Linux, `libnotify`)

On Arch Linux, the package names are `git`, `go`, `python`, and `libsecret`. setup does not install system packages.

setup checks the prerequisites and destination, downloads pinned Go modules, and builds the plugin. It installs only the verified build artifact to `~/.config/omarchy/plugins/io.github.komagata.line`, then rescans and enables it. If the destination already exists or a plugin with the same ID is present, setup stops before building. It does not restart the shell or sign you in.

Cloning the source alone does not create `bin/line-gui`, so the standard `omarchy plugin add` command cannot install it. There are no runtime downloads or builds, and Node, Python, qrencode, and an external `line` CLI are not needed.

## Usage

Open the plugin from the LINE icon in the bar, then reload or sign in with a QR code in Settings. Press Enter to send and Shift+Enter for a line break. Chats are limited to 500 items, and history to the most recent 100. Conversations and drafts are held in memory and are cleared when the service stops. If the result of a send is unknown, the message is not automatically resent.

With the local Go version, chats, icons, owned stickers, and the incoming connection have been checked using a saved account. Repeating a real QR login, sending real messages, and delivery to recipients have not been verified.

![Screenshot of the running app showing a fictional conversation (captured before the demo entry point was removed).](preview.png)

The names, conversation, and icons in the image are fictional. The regular UI has no button for switching to the demo.

## Updating and removal

The installed plugin is a built copy. Update it from the cloned `line` directory; do not use the update operation intended for Git-managed plugins.

```bash
git pull --ff-only
(cd backend && GOTOOLCHAIN=local go mod download)
./scripts/build
omarchy plugin disable io.github.komagata.line
./scripts/install "$HOME/.config/omarchy/plugins/io.github.komagata.line" --upgrade
omarchy-shell shell rescanPlugins
omarchy plugin enable io.github.komagata.line
```

The previous version is kept in `~/.config/omarchy/plugins-backups/`, outside the plugin discovery path, and is restored if replacement fails. To roll back, disable the plugin and specify the backup path printed by the installer.

```bash
./scripts/install "$HOME/.config/omarchy/plugins/io.github.komagata.line" --rollback "$HOME/.config/omarchy/plugins-backups/io.github.komagata.line.backup-..."
omarchy-shell shell rescanPlugins
omarchy plugin enable io.github.komagata.line
```

If setup fails while rescanning or enabling the plugin after installation, fix the cause and run the rescan and enable commands above. Running setup again will not replace existing files.

To remove the plugin, run:

```bash
omarchy plugin disable io.github.komagata.line
omarchy plugin remove io.github.komagata.line
```

Authentication data, the old CLI, and backups are left in place. Review the backup contents before deleting them manually.

## Development and provenance

Tests also require `qmltestrunner` / QtTest. After downloading modules, builds and tests run offline, with the Go cache stored outside the source tree.

```bash
export GOCACHE=/tmp/omarchy-line-gocache
./scripts/build
./tests/run
omarchy plugin validate build/plugin
```

For development UI checks, a demo IPC using fictional data is available. It does not sign in or send real messages. To exit the demo and return to normal use, disable and re-enable the plugin to restart the service.

```bash
omarchy-shell shell summon io.github.komagata.line '{"demo":true}'
```

`build/plugin/` contains QML, the manifest, licenses and provenance, and the statically linked `bin/line-gui`. The build uses `CGO_ENABLED=0`, `-trimpath`, and `-buildvcs=false`. Tests include Go race, vet, and crypto tests, QtTest, and local HTTP verification of message rendering.

See [source and dependency provenance](docs/go-source-provenance.txt). License documents for upstream sources, generated code and data, the Go standard library, and runtime dependencies are included in the distributed artifact's NOTICE.
