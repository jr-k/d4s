<div align="left" width="100%">
    <img src="./docs/img/d4s-unpadded.png" width="328" alt="" />
</div>


# D-Force (d4s)

D4S (pronounced *D-Force*) brings the power and ergonomics of K9s to the local Docker ecosystem. Stop wrestling with verbose CLI commands and start managing your containers like a pro.

<a target="_blank" href="https://github.com/jr-k/d4s/commit/HEAD"><img src="https://img.shields.io/github/last-commit/jr-k/d4s?color=green" /></a>
<a target="_blank" href="https://github.com/jr-k/d4s/stargazers"><img src="https://img.shields.io/github/stars/jr-k/d4s?style=flat&color=yellow" /></a>
<a target="_blank" href="https://github.com/jr-k/d4s/pkgs/container/d4s"><img src="https://img.shields.io/badge/ghcr.io-d4s-orange?logo=github&color=orange" /></a>

## Screenshots
<div align="left" width="100%">
    <img src="./docs/img/screen1.png" width="100%" alt="" />
</div>
<br />
<div align="left" width="100%">
    <img src="./docs/img/screen2.png" width="100%" alt="" />
</div>

## Features

- **Fancy UI**: Modern TUI with Dracula theme, smooth navigation, and live updates.
- **Keyboard Centric**: Vim-like navigation (`j`/`k`), shortcuts for everything. No mouse needed.
- **Full Scope**: Supports **Containers**, **Images**, **Volumes**, **Networks**.
- **Compose Aware**: Easily identify containers belonging to Compose stacks.
- **Swarm Aware**: Supports **Nodes**, **Services**.
- **Powerful Search**: Instant fuzzy filtering (`/`) and command palette (`:`).
- **Live Stats**: Real-time CPU/Mem usage for containers and host context.
- **Advanced Logs**: Streaming logs with auto-scroll, timestamps toggle, and wrap mode.
- **Quick Shell**: Drop into a container shell (`s`) in a split second.
- **Contextual Actions**: Inspect, Restart, Stop, Prune, Delete with safety confirmations.

## Installation

> ### Generic

<details>
<summary><b>Binary Releases</b></summary>

> Automated
```bash
curl -fsSL https://d4scli.io/install.sh | sh -s -- ~/.local/bin
```
*The script installs downloaded binary to `$HOME/.local/bin` directory by default, but it can be changed by setting DIR environment variable.*

> Manual

Grab a release from the [releases page](https://github.com/jr-k/d4s/releases) and install it manually.
</details>

<details>
<summary><b>Docker</b></summary>

```bash
docker run --rm --pull always -it -v /var/run/docker.sock:/var/run/docker.sock ghcr.io/jr-k/d4s:latest
```

**You might want to create an alias for quicker usage. For example:**

```bash
echo "alias d4s='docker run --rm --pull always -it -v /var/run/docker.sock:/var/run/docker.sock ghcr.io/jr-k/d4s:latest'" >> ~/.zshrc
```
*After running this, either restart your terminal or run `source ~/.zshrc` (or `source ~/.bashrc` for Bash) to enable the alias.*
</details>

<details>
<summary><b>From Source</b></summary>

>Requirement: Go 1.21+
```bash
git clone https://github.com/jr-k/d4s.git
cd d4s
go build -o d4s cmd/d4s/main.go
sudo mv d4s ~/.local/bin/
```

```bash
# Make the binary accessible then run it
mv d4s ~/.local/bin/
d4s

# Quickly run from source
go run cmd/d4s/main.go
```
</details>


> ### macOS

<details>
<summary><b>Homebrew</b></summary>

```bash
brew install jr-k/d4s/d4s
```
</details>

> ### Linux

<details>
<summary><b>APT (Debian/Ubuntu)</b></summary>

```bash
sudo apt install -y gpg && curl -fsSL https://apt.d4scli.io/d4s.gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/d4s.gpg
echo "deb [signed-by=/usr/share/keyrings/d4s.gpg] https://apt.d4scli.io stable main" | sudo tee /etc/apt/sources.list.d/d4s.list
sudo apt update
sudo apt install d4s
```
</details>

<details>
<summary><b>RPM (Fedora/RHEL)</b></summary>

```bash
sudo tee /etc/yum.repos.d/d4s.repo <<EOF
[d4s]
name=D4S Repository
baseurl=https://rpm.d4scli.io
enabled=1
gpgcheck=1
gpgkey=https://rpm.d4scli.io/RPM-GPG-KEY-d4s
EOF
sudo dnf install d4s
```
</details>

<details>
<summary><b>Zypper (openSUSE)</b></summary>

```bash
sudo zypper addrepo https://zypper.d4scli.io d4s
sudo zypper refresh
sudo zypper install d4s
```
</details>

> ### Windows

<details>
<summary><b>Scoop</b></summary>

```powershell
scoop bucket add d4s https://github.com/jr-k/scoop-d4s
scoop install d4s
```
</details>


## Usage
```bash
d4s
d4s version
d4s --context my-remote-ctx
```

## Configuration

D4S uses a YAML configuration file located at `$XDG_CONFIG_HOME/d4s/config.yaml` (defaults to `~/.config/d4s/config.yaml`).

All settings are optional and have sensible defaults. Below is a fully documented example:

```yaml
d4s:
  # Refresh interval in seconds. Minimum 2.0 — values below are capped. Default: 2.0
  refreshRate: 2
  # Docker API server request timeout. Default: 120s
  apiServerTimeout: 15s
  # Disable all modification commands (delete, kill, restart, etc.). Default: false
  readOnly: false
  # Default view on startup (containers, images, volumes, networks, services, nodes, compose, secrets). Default: "" (containers)
  defaultView: ""
  # When true, Ctrl+C won't exit — use :quit instead. Default: false
  noExitOnCtrlC: false
  # Skip checking GitHub for new releases on startup. Default: false
  skipLatestRevCheck: false

  # UI settings
  ui:
    # Enable mouse support. Default: false
    enableMouse: false
    # Hide the entire header bar (stats + shortcuts + logo). Default: false
    headless: false
    # Hide the D4S ASCII logo from the header. Default: false
    logoless: false
    # Hide breadcrumb trail in the status bar. Default: false
    crumbsless: false
    # Invert all theme colors (dark↔light), preserving hue. Default: false
    invert: false
    # Skin name — loads from $XDG_DATA_HOME/d4s/skins/<name>.yaml. Default: "default" (builtin: default, dracula)
    skin: "default"

  # Log viewer settings
  logger:
    # Number of tail lines to fetch initially. Default: 100
    tail: 200
    # How far back to go in the log timeline (seconds). -1 = tail mode. Default: -1
    sinceSeconds: -1
    # Enable line wrapping in log viewer. Default: false
    textWrap: false
    # Disable auto-scroll when new log lines arrive. Default: false
    disableAutoscroll: false
    # Show timestamps on each log line. Default: false
    showTime: false

  # Shell pod used for volume browsing and secret decoding
  shellPod:
    image: ghcr.io/jr-k/nget:latest
```

## Contributing

There's still plenty to do! Take a look at the [contributing guide](CONTRIBUTING.md) to see how you can help.

## Discussion / Need help ?

### Open an Issue
[<img src="./docs/img/social/github.png" width="64">](https://github.com/jr-k/d4s/issues/new/choose)

---
*Built with Go & Tview. Inspired by K9s.*

*D4s uses several open source libraries. Thanks to the maintainers who make this possible.*

