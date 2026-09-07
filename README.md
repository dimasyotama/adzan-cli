# adzan

Prayer times in your terminal, with the adhan sounding at the right moment.

A background daemon watches the clock and plays the call to prayer; the CLI
gives you a live dashboard and the commands to control it. Two static Go
binaries, no Electron, no Python, no third-party deps.

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

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh
```

Detects your OS/arch, downloads the release, checks for an audio player, and
warns if the install dir is not on your PATH.

macOS via Homebrew instead:

```sh
brew tap dimasyotama/adzan-cli https://github.com/dimasyotama/adzan-cli
brew install adzan
```

Or build from source (Go 1.22+, no network deps): `git clone` this repo,
`make install`.

Windows has no installer yet — build both binaries with `go build` and put
them in one directory on your PATH. `adzan install` (auto-start) and
`adzan tray` aren't wired up on Windows.

Installer flags: `--version X.Y.Z`, `--prefix /path`, `--from-source`
(pass after `-s --`).

## Activate

```sh
adzan setup     # pick location + calculation method
adzan start     # run the daemon in the background
adzan           # live dashboard
adzan doctor    # check config/audio/daemon are all OK
adzan test      # play the adhan now, to check you can hear it
```

`adzan setup` offers to enable this for you. To do it later, or again:

```sh
adzan install   # systemd user unit (Linux) or launchd agent (macOS) -
                # written, enabled and started in one step, survives reboot
```

## Commands

| Command | What it does |
| --- | --- |
| `adzan` | Live dashboard (starts daemon if needed) |
| `adzan setup` | Choose location and calculation method |
| `adzan start` / `stop` / `quit` | Start daemon / silence current adhan / stop daemon |
| `adzan status` / `times` | One-shot status / today's times |
| `adzan mute` / `unmute` | Stop / resume announcing prayers |
| `adzan test` | Play the adhan now |
| `adzan reload` | Re-read config after hand-editing it |
| `adzan install` / `uninstall` | Add/remove the login service |
| `adzan tray install` / `uninstall` | Menu bar / tray icon (Linux, macOS) |
| `adzan doctor` | Health check |
| `adzan update` | Update to latest release |
| `adzan remove` | Uninstall everything (`--keep-config`, `--yes`) |

## Uninstall

```sh
adzan remove
```

Shows what it will delete, asks before touching anything. Add
`--keep-config` to keep your location and cached times.

## How prayer times work

Computed locally from solar position (no API, no network, no key) — see
`internal/prayer`. `adzan setup` offers Kemenag, MWL, ISNA, Umm al-Qura,
Egyptian, Karachi, JAKIM, Diyanet. If your local mosque is off by a minute or
two, nudge it in `~/.config/adzan/config.json`:

```json
"tune": { "Sunrise": -2, "Maghrib": 1, "Isha": -3 }
```

then `adzan reload`.

## Files

Everything lives under `~/.config/adzan` (`%USERPROFILE%\.config\adzan` on
Windows; move it with `XDG_CONFIG_HOME`): `config.json`, `schedule.json`
(cached times), `sounds/`, `adzan.log`, `adzan.pid`, plus a Unix socket
(`adzan.sock`) or, on Windows, a loopback TCP port (`adzan.port`) for
CLI-to-daemon commands.

## Troubleshooting

Start with `adzan doctor` — it flags config/audio/daemon problems directly.

- **No sound** — no supported player found. Linux: `sudo apt install ffmpeg`
  (or `mpg123`). macOS ships `afplay`. Windows needs the Media Feature Pack,
  or point `sound_path` at a `.wav`.
- **Times off by exactly 1 hour** — timezone; re-run `adzan setup`.
- **Times off by 20+ min** — wrong coordinates; check with `adzan status`,
  re-run `adzan setup` with exact coordinates instead of a city name.
- **Times off by 1-4 min** — normal authority-to-authority variance, fix with
  `tune` above.
- **Garbled dashboard** — terminal doesn't handle ANSI; use `adzan status`
  for plain output, or `NO_COLOR=1 adzan`.

## Architecture

```
cmd/adzan          CLI + terminal UI
cmd/adzand          background daemon (no HTTP/TLS)

internal/config     on-disk config, shared paths
internal/prayer      solar position + prayer-time calc, schedule cache
internal/audio       process-based playback, per-OS backend
internal/ipc         socket/TCP transport, one JSON line per message
internal/daemon      scheduler loop + command handler
internal/ui          ANSI wizard + dashboard, no TUI framework
internal/service     systemd/launchd unit generation
internal/spawn       detached process launch
```

Daemon sleeps until the next prayer (no polling), wakes to play, sleeps
again. CLI is stateless — every command is a round trip over the socket.

## Development

```sh
make test     # unit tests
make build    # binaries into ./bin
make release  # cross-compiled tarballs, all platforms
make deb      # .deb packages
make formula  # regenerate Formula/adzan.rb
```

For a full release, use `make dist VERSION=X.Y.Z` (runs release+deb+formula
together — running them separately wipes `dist/` out from under each other).

## Note on the bundled audio

The repo ships one adhan recording. If you fork/publish this, make sure you
have the right to redistribute whatever audio you bundle.
