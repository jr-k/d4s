<div align="left" width="100%">
    <img src="./docs/img/d4s.png" width="357" alt="" />
</div>


# D-Force (d4S) 🍊

> **The K9s experience for Docker.**  
> Manage your Docker Swarm, Compose stacks, and Containers with a fancy, fast, and keyboard-centric Terminal User Interface.

D4S (pronounced *D-Force*) brings the power and ergonomics of K9s to the local Docker ecosystem. Stop wrestling with verbose CLI commands and start managing your containers like a pro.

## ✨ Features

- 🍊 **Fancy UI**: Modern TUI with Dracula theme, smooth navigation, and live updates.
- ⌨️ **Keyboard Centric**: Vim-like navigation (`j`/`k`), shortcuts for everything. No mouse needed.
- 🐳 **Full Scope**: Supports **Containers**, **Images**, **Volumes**, **Networks**.
- 📦 **Compose Aware**: Easily identify containers belonging to Compose stacks.
- 🐝 **Swarm Aware**: Easily identify nodes and services.
- 🔍 **Powerful Search**: Instant fuzzy filtering (`/`) and command palette (`:`).
- 📊 **Live Stats**: Real-time CPU/Mem usage for containers and host context.
- 📜 **Advanced Logs**: Streaming logs with auto-scroll, timestamps toggle, and wrap mode.
- 🐚 **Quick Shell**: Drop into a container shell (`s`) in a split second.
- 🛠 **Contextual Actions**: Inspect, Restart, Stop, Prune, Delete with safety confirmations.

## 🚀 Installation

### From Source
Requirement: Go 1.21+

```bash
git clone https://github.com/jessym/d4s.git
cd d4s
go build -o d4s cmd/d4s/main.go
sudo mv d4s /usr/local/bin/
```

### Quick Run
```bash
go run cmd/d4s/main.go
```

## 🍊 Mascotte

Meet **Citrus**, our vitamin-packed helper ensuring your containers stay fresh and healthy! 🍊

---
*Built with Go & Tview. Inspired by the legendary K9s.*
