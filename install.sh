#!/bin/sh
# adzan installer
#
#   curl -fsSL https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh | sh
#
# Options (pass after `-s --`, e.g. `... | sh -s -- --version 0.2.0`):
#   --version <v>   install a specific release instead of the latest
#   --prefix <dir>  install somewhere other than ~/.local/bin
#   --from-source   build with Go instead of downloading a release
#
# POSIX sh on purpose: this has to run under dash, ash and busybox, not just bash.

set -eu

REPO="dimasyotama/adzan-cli"
PREFIX="${ADZAN_PREFIX:-$HOME/.local}"
VERSION=""
FROM_SOURCE=0

# ---------- output helpers ----------

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    C_GREEN=$(printf '\033[38;5;42m')
    C_AMBER=$(printf '\033[38;5;214m')
    C_RED=$(printf '\033[38;5;203m')
    C_DIM=$(printf '\033[2m')
    C_BOLD=$(printf '\033[1m')
    C_OFF=$(printf '\033[0m')
else
    C_GREEN=""; C_AMBER=""; C_RED=""; C_DIM=""; C_BOLD=""; C_OFF=""
fi

say()  { printf '  %s\n' "$*"; }
ok()   { printf '  %sOK%s %s\n' "$C_GREEN" "$C_OFF" "$*"; }
warn() { printf '  %s!%s  %s\n' "$C_AMBER" "$C_OFF" "$*"; }
die()  { printf '  %sx%s  %s\n' "$C_RED" "$C_OFF" "$*" >&2; exit 1; }
step() { printf '\n  %s%s%s\n' "$C_BOLD" "$*" "$C_OFF"; }

banner() {
    printf '%s' "$C_GREEN"
    cat <<'ART'

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

ART
    printf '%s' "$C_OFF"
    printf '  %sadzan%s %s\n' "$C_BOLD" "$C_OFF" "${C_DIM}prayer times in your terminal$C_OFF"
}

# A spinner for the slow steps, so the install never looks frozen.
spin() {
    _msg="$1"; shift
    if [ ! -t 1 ]; then
        say "$_msg"
        "$@"
        return $?
    fi

    "$@" &
    _pid=$!
    _frames='- \ | /'
    while kill -0 "$_pid" 2>/dev/null; do
        for _f in $_frames; do
            printf '\r  %s%s%s %s%s%s' "$C_AMBER" "$_f" "$C_OFF" "$C_DIM" "$_msg" "$C_OFF"
            sleep 0.1
            kill -0 "$_pid" 2>/dev/null || break
        done
    done
    wait "$_pid"
    _rc=$?
    printf '\r%*s\r' 60 ''
    return $_rc
}

# ---------- argument parsing ----------

while [ $# -gt 0 ]; do
    case "$1" in
        --version) VERSION="${2:-}"; shift 2 ;;
        --prefix)  PREFIX="${2:-}";  shift 2 ;;
        --from-source) FROM_SOURCE=1; shift ;;
        -h|--help)
            sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
            exit 0 ;;
        *) die "unknown option: $1" ;;
    esac
done

# ---------- platform detection ----------

detect_platform() {
    _os=$(uname -s)
    _arch=$(uname -m)

    case "$_os" in
        Linux)  OS="linux" ;;
        Darwin) OS="darwin" ;;
        MINGW*|MSYS*|CYGWIN*)
            die "Windows is not supported by this script - see the README for PowerShell instructions" ;;
        *) die "unsupported operating system: $_os" ;;
    esac

    case "$_arch" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) die "unsupported architecture: $_arch (build from source with --from-source)" ;;
    esac
}

# ---------- dependency checks ----------

have() { command -v "$1" >/dev/null 2>&1; }

check_downloader() {
    if have curl; then
        DL="curl"
    elif have wget; then
        DL="wget"
    else
        die "neither curl nor wget is installed"
    fi
}

fetch() {
    # fetch <url> <output-path>
    if [ "$DL" = "curl" ]; then
        curl -fsSL "$1" -o "$2"
    else
        wget -qO "$2" "$1"
    fi
}

fetch_stdout() {
    if [ "$DL" = "curl" ]; then
        curl -fsSL "$1"
    else
        wget -qO- "$1"
    fi
}

# adzan plays audio by calling a player that is already on the system, so warn
# rather than fail if none is present - the user can install one later.
check_audio() {
    if [ "$OS" = "darwin" ]; then
        if have afplay; then
            ok "audio: afplay"
            return
        fi
    fi
    for p in ffplay mpv mpg123 cvlc paplay aplay; do
        if have "$p"; then
            ok "audio: $p"
            return
        fi
    done
    warn "no audio player found - install one so the adhan can play:"
    if [ "$OS" = "linux" ]; then
        say "     ${C_DIM}sudo apt install ffmpeg${C_OFF}   or   ${C_DIM}sudo apt install mpg123${C_OFF}"
    fi
}

# ---------- install paths ----------

resolve_latest() {
    if [ -n "$VERSION" ]; then
        return
    fi
    _api="https://api.github.com/repos/$REPO/releases/latest"
    VERSION=$(fetch_stdout "$_api" 2>/dev/null \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' \
        | head -n 1) || true
}

install_from_release() {
    _tarball="adzan-$VERSION-$OS-$ARCH.tar.gz"
    _url="https://github.com/$REPO/releases/download/v$VERSION/$_tarball"

    TMP=$(mktemp -d)
    trap 'rm -rf "$TMP"' EXIT INT TERM

    spin "Downloading $_tarball" fetch "$_url" "$TMP/$_tarball" \
        || die "download failed: $_url"

    spin "Extracting" tar -xzf "$TMP/$_tarball" -C "$TMP" \
        || die "could not extract $_tarball"

    _src=$(find "$TMP" -type f -name adzan -perm -u+x | head -n 1)
    [ -n "$_src" ] || die "the archive did not contain an adzan binary"
    _dir=$(dirname "$_src")

    mkdir -p "$PREFIX/bin"
    install -m 0755 "$_dir/adzan"  "$PREFIX/bin/adzan"
    install -m 0755 "$_dir/adzand" "$PREFIX/bin/adzand"
    if [ -f "$_dir/adzantray" ]; then
        install -m 0755 "$_dir/adzantray" "$PREFIX/bin/adzantray"
    else
        warn "this release has no adzantray for $OS/$ARCH - 'adzan tray' won't be available (build with --from-source on a Mac with Xcode tools to get it on darwin)"
    fi
}

install_from_source() {
    have go || die "Go is not installed - see https://go.dev/dl (or install a release instead)"

    TMP=$(mktemp -d)
    trap 'rm -rf "$TMP"' EXIT INT TERM

    spin "Cloning $REPO" git clone --depth 1 "https://github.com/$REPO.git" "$TMP/src" \
        || die "git clone failed"

    mkdir -p "$PREFIX/bin"
    spin "Building adzan" sh -c "cd '$TMP/src' && go build -trimpath -ldflags '-s -w' -o '$PREFIX/bin/adzan' ./cmd/adzan" \
        || die "build failed"
    spin "Building adzand" sh -c "cd '$TMP/src' && go build -trimpath -ldflags '-s -w' -o '$PREFIX/bin/adzand' ./cmd/adzand" \
        || die "build failed"
    spin "Building adzantray" sh -c "cd '$TMP/src' && go build -trimpath -ldflags '-s -w' -o '$PREFIX/bin/adzantray' ./cmd/adzantray" \
        || warn "adzantray build failed - 'adzan tray' won't be available (on darwin this needs Xcode command line tools for cgo)"
}

# The two binaries must live together: the CLI looks for the daemon beside itself.
verify() {
    [ -x "$PREFIX/bin/adzan" ]  || die "adzan was not installed correctly"
    [ -x "$PREFIX/bin/adzand" ] || die "adzand was not installed correctly"
}

path_hint() {
    case ":$PATH:" in
        *":$PREFIX/bin:"*)
            return ;;
    esac

    warn "$PREFIX/bin is not on your PATH. Add it with:"
    _shell=$(basename "${SHELL:-sh}")
    case "$_shell" in
        zsh)  say "     ${C_DIM}echo 'export PATH=\"$PREFIX/bin:\$PATH\"' >> ~/.zshrc && source ~/.zshrc${C_OFF}" ;;
        fish) say "     ${C_DIM}fish_add_path $PREFIX/bin${C_OFF}" ;;
        *)    say "     ${C_DIM}echo 'export PATH=\"$PREFIX/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc${C_OFF}" ;;
    esac
}

# ---------- main ----------

banner
detect_platform
check_downloader

step "Checking your system"
ok "platform: $OS/$ARCH"
check_audio

step "Installing"

if [ "$FROM_SOURCE" -eq 1 ]; then
    install_from_source
    VERSION="source"
else
    resolve_latest
    if [ -z "$VERSION" ]; then
        warn "no published release found - building from source instead"
        install_from_source
        VERSION="source"
    else
        install_from_release
    fi
fi

verify
ok "installed adzan and adzand to $PREFIX/bin"
path_hint

step "Next steps"
say "${C_DIM}adzan setup${C_OFF}   choose your location"
say "${C_DIM}adzan start${C_OFF}   run it in the background"
say "${C_DIM}adzan${C_OFF}         live dashboard"
printf '\n'
