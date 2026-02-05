<div align="left" width="100%">
    <img src="./docs/img/d4s-unpadded.png" width="328" alt="" />
</div>


# D-Force (d4s)

D4S (pronounced *D-Force*) brings the power and ergonomics of K9s to the local Docker ecosystem. Stop wrestling with verbose CLI commands and start managing your containers like a pro.

> Manage your Docker Swarm, Compose stacks, and Containers with a fancy, fast, and keyboard-centric Terminal User Interface.


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
curl -fsSL https://apt.d4scli.io/d4s.gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/d4s.gpg
echo "deb [signed-by=/usr/share/keyrings/d4s.gpg] https://apt.d4scli.io stable main" | sudo tee /etc/apt/sources.list.d/d4s.list
sudo apt update && sudo apt install d4s
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
<summary><b>Pacman (Arch Linux)</b></summary>

```bash
sudo tee -a /etc/pacman.conf <<EOF

[d4s]
SigLevel = Optional TrustAll
Server = https://pacman.d4scli.io
EOF
sudo pacman -Sy d4s
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

<details>
<summary><b>APK (Alpine)</b></summary>

```bash
echo "https://apk.d4scli.io/v3.18/main" >> /etc/apk/repositories
apk update && apk add d4s
```
</details>

<details>
<summary><b>XBPS (Void Linux)</b></summary>

```bash
echo "repository=https://xbps.d4scli.io/current" | sudo tee /etc/xbps.d/d4s.conf
sudo xbps-install -S d4s
```
</details>

<details>
<summary><b>Emerge (Gentoo)</b></summary>

```bash
# Add to /etc/portage/binrepos.conf
sudo tee /etc/portage/binrepos.conf/d4s.conf <<EOF
[d4s]
sync-uri = https://emerge.d4scli.io/packages
EOF
sudo emerge --sync
sudo emerge -G d4s
```
</details>

<details>
<summary><b>OPKG (OpenWrt)</b></summary>

```bash
echo "src/gz d4s https://opkg.d4scli.io" >> /etc/opkg/customfeeds.conf
opkg update && opkg install d4s
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
```

## Contributing

There's still plenty to do! Take a look at the [contributing guide](CONTRIBUTING.md) to see how you can help.

## Discussion / Need help ?

### Join our Discord
[<img src="./docs/img/social/discord.png" width="64">](https://discord.gg/tS2NCEJTUN)

### Open an Issue
[<img src="./docs/img/social/github.png" width="64">](https://github.com/jr-k/d4s/issues/new/choose)

---
*Built with Go & Tview. Inspired by K9s.*

*D4s uses several open source libraries. Thanks to the maintainers who make this possible.*

