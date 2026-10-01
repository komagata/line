# LINE for Omarchy

A work-in-progress QML and Go plugin for using LINE chats, replies, attachments, and stickers in Omarchy 4 Quattro. This is not an official LINE product.

## Installation

Install with Omarchy's Git-managed plugin support. A marketplace listing is not needed. Setup builds the backend, installs its executable and license notice, registers the app launcher, and enables the plugin:

```bash
omarchy plugin add https://github.com/komagata/line.git --yes
~/.config/omarchy/plugins/io.github.komagata.line/scripts/setup --in-place
```

`--yes` accepts the trusted repository clone confirmation; adding the plugin leaves it disabled. Prepare these requirements before running the commands:

- Linux, Omarchy 4 Quattro / Quickshell, Qt Quick Controls and Dialogs
- For fetching and building the source: git, Go 1.26 or later, Python 3
- For storing authentication: libsecret's `/usr/bin/secret-tool` and an unlocked Secret Service
- For notifications: `/usr/bin/notify-send` (on Arch Linux, `libnotify`)

On Arch Linux, the package names are `git`, `go`, `python`, and `libsecret`. setup does not install system packages.

`omarchy plugin add` clones, validates, and rescans; it does not build the backend. `setup --in-place` checks prerequisites and the canonical installed Git checkout, disables it, downloads pinned Go modules, builds and validates the payload, and replaces only `bin/line-gui` and `NOTICE`. It preserves Git metadata and source files. Setup rescans, registers the launcher, then enables the plugin last. After disabling succeeds, any failure leaves the plugin disabled; fix the cause and rerun the same setup command. The two generated files are each replaced atomically; a replacement failure can leave one updated, so rerun setup before enabling. Setup does not install system packages, restart the shell, or sign you in.

<details>
<summary>Alternative: source-copy installation</summary>

Prepare the same prerequisites, then run:

```bash
git clone https://github.com/komagata/line.git && cd line && ./scripts/setup
```

Without `--in-place`, setup installs only the verified build artifact to `~/.config/omarchy/plugins/io.github.komagata.line`, rescans, registers the launcher, and enables it last. If the destination already exists or a plugin with the same ID is present, setup stops before building. This installation does not retain a Git checkout at the destination; use the source-copy update and rollback steps below.

</details>

The launcher uses the standard `internet-chat` themed icon and opens the plugin with `omarchy-shell shell summon io.github.komagata.line`. Its desktop entry is stored under `${XDG_DATA_HOME:-$HOME/.local/share}/applications/`. An unrelated file or symlink at that path is refused. If `update-desktop-database` is available, the helper refreshes it; a refresh failure is reported as a warning after the desktop entry change succeeds.

There are no runtime downloads or builds, and Node, Python, qrencode, and an external `line` CLI are not needed.

## Usage

Search for LINE in the app launcher or open it from the LINE icon in the bar, then reload or sign in with a QR code in Settings. Press Enter to send and Shift+Enter for a line break. Chats are limited to 500 items, and history to the most recent 100. Conversations and drafts are held in memory and are cleared when the service stops. If the result of a send is unknown, the message is not automatically resent.

By default, the UI follows the system Qt locale: Japanese locales use Japanese; English, unsupported, empty, and C/POSIX locales use English. Choose **日本語** or **English** in Settings to save an explicit override, or **System default / システム設定** to return to automatic detection. Invalid saved preferences also fall back to system detection. Automatic detection is not saved as an explicit override. Changes apply immediately and keep the current conversation, draft, and login state. The language preference is saved in `${XDG_CONFIG_HOME:-$HOME/.config}/omarchy-line/preferences.ini`, separately from the plugin and authentication. Removing or updating the plugin keeps this preference; delete that file to reset the language. Other settings, including notifications, remain session-only.

With the local Go version, chats, icons, owned stickers, and the incoming connection have been checked using a saved account. Repeating a real QR login, sending real messages, and delivery to recipients have not been verified.

![English UI with a fictional demo conversation.](preview.png)

The names, conversation, and icons in the English demo screenshot are fictional. The regular UI has no button for switching to the demo.

To prepare screenshots from the actual ChatView without signing in or changing the running shell:

```bash
./scripts/capture-preview /tmp/omarchy-line-english-preview
```

This requires the build prerequisites and `qmltestrunner`. It derives fixtures from the Go demo, selects English through Settings, and saves chat and Settings PNGs at 1200×760 and 720×540. The offscreen QtTest harness uses isolated HOME/XDG paths and a stub backend process; it performs no live LINE operations and does not replace `preview.png`. Inspect the captures before publishing them.

## Updating and removal

### Git-managed installation

Disable the plugin before updating its installed checkout, then rebuild with setup:

```bash
(set -e
  omarchy plugin disable io.github.komagata.line
  omarchy plugin update io.github.komagata.line
  ~/.config/omarchy/plugins/io.github.komagata.line/scripts/setup --in-place
)
```

`omarchy plugin update` fetches and fast-forwards the checkout but does not build the backend. It may ask for permission to update the repository. If updating or setup fails after disabling succeeds, the plugin remains disabled. Fix the cause and rerun the sequence. Setup can also rebuild the current checkout without updating it.

### Source-copy installation

For the source-copy alternative, update it from the cloned `line` source directory; do not use the update operation intended for Git-managed plugins.

```bash
git pull --ff-only
(cd backend && GOTOOLCHAIN=local go mod download)
./scripts/build
omarchy plugin disable io.github.komagata.line
./scripts/install "$HOME/.config/omarchy/plugins/io.github.komagata.line" --upgrade
omarchy-shell shell rescanPlugins
omarchy plugin enable io.github.komagata.line
./scripts/launcher install
```

For a source-copy installation, register or refresh the app launcher from the cloned `line` directory without rebuilding:

```bash
./scripts/launcher install
```

The helper updates only an entry marked as its own or one whose `Exec` exactly matches the plugin summon command. It does not enable the plugin or change its files. `scripts/install` remains an explicit file installer and does not register a launcher.

The previous version is kept in `~/.config/omarchy/plugins-backups/`, outside the plugin discovery path, and is restored if replacement fails. To roll back, disable the plugin and specify the backup path printed by the installer.

```bash
./scripts/install "$HOME/.config/omarchy/plugins/io.github.komagata.line" --rollback "$HOME/.config/omarchy/plugins-backups/io.github.komagata.line.backup-..."
omarchy-shell shell rescanPlugins
omarchy plugin enable io.github.komagata.line
```

If source-copy setup fails after installing files, the plugin has not been enabled by setup. Fix the cause, then run these commands from the cloned `line` directory; running setup again will not replace the existing installation:

```bash
(set -e
  omarchy-shell shell rescanPlugins
  ./scripts/launcher install
  omarchy plugin enable io.github.komagata.line
)
```

To remove a Git-managed installation, remove its launcher entry, disable it, then remove the plugin:

```bash
(set -e
  cd "$HOME/.config/omarchy/plugins/io.github.komagata.line"
  ./scripts/launcher remove
  omarchy plugin disable io.github.komagata.line
  omarchy plugin remove io.github.komagata.line
)
```

For a source-copy installation, run `./scripts/launcher remove` from the cloned `line` directory, then disable and remove the installed plugin as above.

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
