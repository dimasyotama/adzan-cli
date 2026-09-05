# adzan

Prayer times in your terminal, with the adhan sounding at the right moment.

A small background daemon watches the clock and plays the call to prayer; a
separate CLI gives you a live dashboard and the handful of commands you need to
control it. No Electron, no Python runtime, no framework — two static Go
binaries and the standard library.

```
                    .
                   /|\
                  ( o )
                   \|/
                    |
      ___       _________       ___
     |   |    ,'         ',    |   |
     | o |   /             \   | o |
     |___|  |               |  |___|
     |   |  |    _______    |  |   |
     |   |  |   /       \   |  |   |
     |   |  |  |  .   .  |  |  |   |
   __|___|__|__|_________|__|__|___|__
  |___________________________________|

  ADZAN   * running
  -----------------------------------------
  location   Purwakarta, Indonesia
  method     Kemenag - Indonesia
  timezone   Asia/Jakarta

  next       Maghrib  02:14:07
             at 17:52 today

    Fajr                04:34
    Sunrise             05:47
    Dhuhr               11:52
    Asr                 15:11
  > Maghrib             17:52
    Isha                19:01

  adzan stop to silence   adzan mute to stay quiet   ctrl-c to exit
```

---

## Quick start

```sh
curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh
adzan setup
adzan start
```

That is the whole install: the script detects your OS and architecture, fetches
the right release, checks you have an audio player, and tells you if the
install directory is not on your PATH.

To remove it again, at any point:

```sh
adzan remove
```

It shows exactly what it will delete and asks before touching anything. Use
`adzan remove --keep-config` to keep your location and cached times.

Options for the installer, if you need them:

```sh
# a specific version
curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh -s -- --version 0.2.0

# somewhere other than ~/.local
curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh -s -- --prefix /usr/local

# build with Go rather than downloading a release
curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh -s -- --from-source
```

The manual, per-OS instructions below are still there if you would rather not
pipe a script into a shell.

---

## Contents

- [Quick start](#quick-start)
- [What you need](#what-you-need)
- [Install on Linux](#install-on-linux)
- [Install on macOS](#install-on-macos)
- [Install on Windows](#install-on-windows)
- [First run](#first-run)
- [Commands](#commands)
- [Where your files live](#where-your-files-live)
- [Prayer times](#prayer-times)
- [Sound selection](#sound-selection)
- [Troubleshooting](#troubleshooting)
- [Architecture](#architecture)
- [Development](#development)
- [Note on the bundled audio](#note-on-the-bundled-audio)

---

## What you need

| | Linux | macOS | Windows |
| --- | --- | --- | --- |
| Install | `install.sh` one-liner | `install.sh` one-liner | build with Go |
| Go (only to build) | 1.22+ | 1.22+ | 1.22+ |
| Network | not needed | not needed | not needed |
| Audio player | `ffplay`, `mpv` or `mpg123` — install one | `afplay`, already there | Windows Media Player, already there |
| Background service | systemd user unit | launchd agent | Task Scheduler (manual) |
| IPC transport | Unix socket | Unix socket | loopback TCP |

Everything else is standard library. There are no third-party Go dependencies,
so `go build` never touches the network.

---

## Install on Linux

### 1. Install Go

```sh
# Debian / Ubuntu
sudo apt update && sudo apt install -y golang-go

# Fedora
sudo dnf install -y golang

# Arch
sudo pacman -S go
```

Check it: `go version` should print 1.22 or newer. If your distro ships
something older, grab the official tarball from https://go.dev/dl instead.

### 2. Install an audio player

The daemon shells out to whatever player you already have. Install one:

```sh
# Debian / Ubuntu — ffmpeg provides ffplay
sudo apt install -y ffmpeg
# or, lighter:
sudo apt install -y mpg123

# Fedora
sudo dnf install -y ffmpeg-free      # or: sudo dnf install -y mpg123

# Arch
sudo pacman -S ffmpeg                # or: sudo pacman -S mpg123
```

### 3. Install

Via a `.deb`, from the release page (also gives you a systemd unit path at
`/usr/bin`, no PATH changes needed):

```sh
curl -fsSLO https://github.com/dimasyotama/adzan-cli/releases/download/vVERSION/adzan_VERSION_amd64.deb
sudo apt install ./adzan_VERSION_amd64.deb   # or arm64, on aarch64
```

Or the one-liner, which handles everything without a `.deb`:

```sh
curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh
```

Or build it yourself:

```sh
git clone https://github.com/dimasyotama/adzan-cli.git
cd adzan
make install
```

Either way both binaries land in `~/.local/bin`. Install elsewhere with
`make install PREFIX=/usr/local` (that one needs `sudo`).

### 4. Put it on your PATH

If `adzan` is not found, add `~/.local/bin` to your PATH:

```sh
# bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc

# zsh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc

# fish
fish_add_path ~/.local/bin
```

### 5. Run it at login

`adzan start` detaches the daemon, which lasts for the session. To have it
survive logout and start automatically, use systemd:

```sh
adzan install
systemctl --user daemon-reload
systemctl --user enable --now adzan
```

Check it, follow its log, or stop it:

```sh
systemctl --user status adzan
journalctl --user -u adzan -f
systemctl --user stop adzan
```

If the daemon should keep running when you are not logged in (a headless box,
for instance), enable lingering once:

```sh
sudo loginctl enable-linger "$USER"
```

---

## Install on macOS

### 1. Install Go

```sh
brew install go
```

No Homebrew? Download the `.pkg` from https://go.dev/dl and run it.

### 2. Audio

Nothing to install. `afplay` ships with macOS and handles MP3 natively. The
daemon will use `ffplay`, `mpv` or `mpg123` instead if you happen to have one.

### 3. Install

Via Homebrew, from the tap:

```sh
brew tap dimasyotama/adzan-cli
brew install adzan
```

Or the one-liner, which handles everything without a tap:

```sh
curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh
```

Or build it yourself:

```sh
git clone https://github.com/dimasyotama/adzan-cli.git
cd adzan
make install
```

Both binaries land in `~/.local/bin`. To use Homebrew's prefix instead:

```sh
make install PREFIX="$(brew --prefix)"
```

### 4. Put it on your PATH

macOS defaults to zsh:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

### 5. Run it at login

```sh
adzan install
launchctl load -w ~/Library/LaunchAgents/com.github.dimasyotama.adzan.plist
```

Check it or unload it:

```sh
launchctl list | grep adzan
launchctl unload -w ~/Library/LaunchAgents/com.github.dimasyotama.adzan.plist
```

On newer macOS you may prefer the modern syntax:

```sh
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.github.dimasyotama.adzan.plist
launchctl bootout   gui/$(id -u)/com.github.dimasyotama.adzan
```

### Gatekeeper

Binaries you compiled yourself are not quarantined, so nothing should block
them. If you download a release tarball instead and macOS refuses to run it:

```sh
xattr -d com.apple.quarantine ./adzan ./adzand
```

### Note on config location

Config goes to `~/.config/adzan`, not `~/Library/Application Support`. This is
deliberate — the same path works across all three platforms, and it is where
CLI tools generally put things.

---

## Install on Windows

Windows works, with three caveats: `adzan install` does not write a Task
Scheduler entry yet (do it manually, below), audio goes through Windows
Media Player via PowerShell rather than a dedicated player, and `adzan tray`
is not wired up yet (no Windows service/login-item support in `service`
today, though the tray binary itself builds and runs there).

### 1. Install Go

```powershell
winget install GoLang.Go
```

Or download the MSI from https://go.dev/dl. Open a **new** terminal afterwards
so `go` is on your PATH.

### 2. Build

There is no `make` by default, so build directly:

```powershell
git clone https://github.com/dimasyotama/adzan-cli.git
cd adzan

go build -trimpath -ldflags "-s -w" -o bin\adzan.exe  .\cmd\adzan
go build -trimpath -ldflags "-s -w" -o bin\adzand.exe .\cmd\adzand
```

### 3. Install

Put both `.exe` files in one directory — the CLI looks for the daemon next to
itself:

```powershell
$dest = "$env:LOCALAPPDATA\Programs\adzan"
New-Item -ItemType Directory -Force -Path $dest
Copy-Item bin\adzan.exe, bin\adzand.exe $dest
```

Add that directory to your PATH for the current user:

```powershell
[Environment]::SetEnvironmentVariable(
  "Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$dest",
  "User"
)
```

Open a new terminal, then check: `adzan version`.

### 4. Run it at login

Register a Task Scheduler entry that starts the daemon when you log in:

```powershell
schtasks /create /tn "adzan" `
  /tr "$env:LOCALAPPDATA\Programs\adzan\adzand.exe" `
  /sc onlogon /rl limited
```

Manage it:

```powershell
schtasks /query  /tn "adzan"
schtasks /run    /tn "adzan"
schtasks /end    /tn "adzan"
schtasks /delete /tn "adzan" /f
```

Without this, `adzan start` still works fine for the current session.

### Terminal note

Use **Windows Terminal** or PowerShell 7. The dashboard uses ANSI escapes, and
the old `conhost` console does not render them well. If you must use the legacy
console, run `adzan status` instead of the live dashboard.

---

## First run

Same on every platform:

```sh
adzan setup     # choose your location and calculation method
adzan start     # run the daemon in the background
adzan           # live dashboard
```

`adzan setup` asks whether you want to give a **city name** or **coordinates**,
then validates your answer against the prayer-times service before saving. A
misspelled city or an impossible coordinate is rejected with a clear message
and you are asked again — nothing invalid ever reaches the daemon.

Verify everything is wired up:

```sh
adzan doctor
```

```
  OK   config   /home/you/.config/adzan/config.json
  OK   location Purwakarta, Indonesia (-6.5569, 107.4431)
  OK   audio    using ffplay
  OK   sound    /home/you/.config/adzan/sounds/default.mp3
  OK   daemon   running
```

Then confirm you can actually hear it:

```sh
adzan test      # plays the adhan now
adzan stop      # silences it
```

---

## Commands

| Command | What it does |
| --- | --- |
| `adzan` | Live dashboard (starts the daemon if it is not running) |
| `adzan setup` | Choose location and calculation method |
| `adzan start` | Start the background daemon |
| `adzan stop` | Silence the adhan that is playing right now |
| `adzan quit` | Stop the background daemon |
| `adzan status` | One-shot status, no live view |
| `adzan times` | Today's prayer times |
| `adzan mute` / `unmute` | Stop / resume announcing prayers |
| `adzan test` | Play the adhan now, to check your audio |
| `adzan reload` | Re-read the config after editing it by hand |
| `adzan install` | Install a systemd user unit or launchd agent |
| `adzan uninstall` | Remove it |
| `adzan tray install` | Show the next prayer in the menu bar / tray (Linux, macOS) |
| `adzan tray uninstall` | Remove it |
| `adzan doctor` | Check audio, config and daemon health |
| `adzan update` | Update to the latest release |
| `adzan remove` | Uninstall adzan completely (`--keep-config`, `--yes`) |
| `adzan version` | Print the version |

`adzan stop` silences the current adhan but leaves the daemon running, so the
next prayer is still announced. Use `adzan quit` to stop the daemon itself.

---

## Where your files live

Everything is under `~/.config/adzan` on Linux and macOS, and
`%USERPROFILE%\.config\adzan` on Windows. Set `XDG_CONFIG_HOME` to move it.

```
config.json      location, calculation method, sound path, mute state
schedule.json    cached prayer times, refreshed a month at a time
sounds/          the adhan audio
adzan.log        daemon output
adzan.pid        daemon process id
adzan.sock       command socket (Linux/macOS; may sit in $XDG_RUNTIME_DIR)
adzan.port       command port    (Windows only)
```

Edit `config.json` by hand if you like, then run `adzan reload`.

Service files land outside that directory, in the place each platform expects:

| Platform | Path |
| --- | --- |
| Linux | `~/.config/systemd/user/adzan.service` |
| macOS | `~/Library/LaunchAgents/com.github.dimasyotama.adzan.plist` |

---

## Prayer times

**Times are computed locally.** No API, no network, no key. Prayer times are
astronomy: the sun's declination and the equation of time for your date, then
hour angles from your latitude and longitude.

- **Fajr** — the sun reaches the method's depression angle below the horizon
- **Sunrise / Maghrib** — the sun's upper limb touches the horizon (0.833 deg,
  allowing for refraction)
- **Dhuhr** — solar noon, when the sun crosses your meridian
- **Asr** — an object's shadow reaches a set multiple of its length
- **Isha** — the sun reaches the method's depression angle after sunset

This means adzan works on a plane, in a datacentre with no egress, or on a
network that blocks the outside world. It also means your times cannot be
wrong because a geocoder matched the wrong city.

Verified against published Kemenag times for Cibitung, Kabupaten Bekasi
(2 September 2026), and that comparison is a test in the suite:

| | computed | published |
| --- | --- | --- |
| Fajr | 04:36 | 04:36 |
| Sunrise | 05:50 | 05:48 |
| Dhuhr | 11:55 | 11:55 |
| Asr | 15:12 | 15:12 |
| Maghrib | 17:53 | 17:54 |
| Isha | 19:03 | 19:00 |

### Calculation methods

`adzan setup` offers Kemenag (Indonesia, the default), Muslim World League,
ISNA, Umm al-Qura, Egyptian, Karachi, JAKIM and Diyanet, plus a Kemenag variant
using the Hanafi position for Asr.

### Tuning

If your local mosque announces times a minute or two off from the computed
ones, nudge individual prayers in `config.json`:

```json
"tune": {
  "Sunrise": -2,
  "Maghrib": 1,
  "Isha": -3
}
```

Values are whole minutes, positive or negative. Run `adzan reload` afterwards.
With the offsets above, the table on the left matches the published times
exactly.

### Why not an API?

Earlier versions fetched from the Aladhan API. Two problems showed up
immediately in the field, and both are structural rather than bad luck:

1. **Networks fail in ways you cannot control.** A working IPv4 path and a
   broken IPv6 one to the same host produced connections that opened and were
   then reset mid-handshake.
2. **A geocoder can put you in the wrong country, silently.** Looking up
   "Cibitung, Indonesia" returned a point in the Andaman Sea, about 2,000 km
   away. The times were wrong by 24 to 46 minutes with no error and no way for
   the program to notice.

The second is the serious one: a network failure is loud, a wrong location is
quiet. Computing locally removes both. A city-name lookup is still offered
during setup as a convenience, but it now shows you the coordinates it resolved
and asks you to confirm them before saving.

### Checking your location

Coordinates are the one input adzan cannot verify for you. If times look wrong,
check them first:

```sh
adzan status     # shows the coordinates in use
```

Long-press your location in any maps app to copy its coordinates, then
`adzan setup` and enter them directly.

## Sound selection

There is one bundled adhan today. Choosing between reciters is deliberately
left for later — the config isolates the sound behind a single `sound_path`
field, so adding a picker means adding a library map and a wizard screen, not
restructuring anything.

To use a different sound now, point `sound_path` at any MP3 you have the right
to use and run `adzan reload`.

---

## Troubleshooting

### Nothing plays at prayer time

Run `adzan doctor` first. If audio shows `FAIL`:

- **Linux** — no supported player found. Install one: `sudo apt install ffmpeg`
  (or `mpg123`). Also check you are not on a headless box with no sound device:
  `ls /dev/snd` should list something.
- **macOS** — `afplay` is missing, which is unusual. Check `which afplay`.
- **Windows** — Windows Media Player is absent. This happens on N/KN editions;
  install the Media Feature Pack, or point `sound_path` at a `.wav` file.

If audio shows `OK` but you still hear nothing, check your volume and run
`adzan test` — it plays immediately and tells you whether the daemon started
playback at all.

### "the daemon is not running"

```sh
adzan start
```

If it will not stay up, read the log:

```sh
cat ~/.config/adzan/adzan.log                          # Linux / macOS
Get-Content $env:USERPROFILE\.config\adzan\adzan.log   # Windows
```

### "another adzan daemon is already running"

One is genuinely running (`adzan status` will confirm), or a socket was left
behind by a crash. The daemon probes and removes stale sockets on its own, so
this almost always means the first case. Stop it with `adzan quit`.

### Times look wrong by an hour or two

Two likely causes. Check `adzan status` shows the timezone you expect — if not,
re-run `adzan setup`. If the timezone is right but the times are off by a few
minutes, you probably want a different calculation method; re-run `adzan setup`
and pick the one your local mosque follows.

### Times are wrong by 20 minutes or more

Almost always the coordinates. Check them:

```sh
adzan status
```

If the numbers are not your actual location, run `adzan setup` and enter
coordinates directly rather than a city name. A wrong location shifts each
prayer by a *different* amount, which is how you tell it apart from a timezone
problem.

### Times are wrong by exactly one hour

That is a timezone, not a location. `adzan status` shows the timezone in use;
re-run `adzan setup` to correct it.

### Times are off by one to four minutes

Different authorities apply different safety margins, so a small constant
difference from your local mosque is normal rather than a bug. Line them up
with the `tune` block described under [Prayer times](#prayer-times).

### The dashboard looks like garbage

Your terminal is not handling ANSI escapes. Use Windows Terminal on Windows.
Anywhere else, set `TERM` correctly, or use `adzan status` for plain output.
`NO_COLOR=1 adzan` disables colour but keeps the layout.

---

## Architecture

```
cmd/adzan     CLI + terminal UI          ~7 MB (2 MB of that is the adhan)
cmd/adzand    background daemon          ~3 MB (no HTTP or TLS linked in)

internal/config    on-disk config and the paths both binaries agree on
internal/prayer    solar position and prayer-time calculation, schedule cache
internal/audio     process-based playback, per-OS backend
internal/ipc       Unix socket (loopback TCP on Windows), one JSON line per message
internal/daemon    the scheduler loop and command handler
internal/ui        ANSI wizard and dashboard, no TUI framework
internal/service   systemd unit / launchd agent generation
internal/spawn     detached process launch
```

The daemon sleeps until the next prayer rather than polling, wakes to play,
then sleeps again. The CLI holds no state: every command is a round trip over
the socket. Prayer times are computed locally, so the daemon makes no network
calls at all once configured.

---

## Development

```sh
make test     # unit tests
make vet
make build    # binaries into ./bin
make size     # stripped sizes
make release  # cross-compiled tarballs for Linux, macOS and Windows
make deb      # .deb packages from the release tarballs (needs dpkg-deb)
make formula  # regenerates Formula/adzan.rb from the release checksums
```

`make release` produces `dist/adzan-<version>-<os>-<arch>.tar.gz` for
linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64.

Cross-compiling `adzan`/`adzand` by hand works too, since there is no CGo:

```sh
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o adzan ./cmd/adzan
```

`adzantray` is the exception: it needs CGo (and Cocoa) on darwin, so it can
only be built natively there. `make release` cross-compiles it for Linux and
Windows (pure Go, no CGo needed) and skips it for darwin - see the comment in
the Makefile.

Cutting a release: tag `vX.Y.Z`, push the tag, run `make release VERSION=X.Y.Z
&& make deb VERSION=X.Y.Z && make formula VERSION=X.Y.Z`, upload everything
under `dist/` (tarballs, `.deb`s, `checksums.txt`) to the GitHub release, and
commit the regenerated `Formula/adzan.rb` to the `dimasyotama/homebrew-adzan-cli` tap.

---

## Note on the bundled audio

The repository ships one adhan recording. If you publish this, make sure you
have the right to redistribute whatever audio you bundle — recordings by named
reciters are generally not openly licensed. Shipping no audio and having
`adzan setup` ask the user for a file is the safest default for a public
release.
