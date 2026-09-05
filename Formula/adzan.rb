class Adzan < Formula
  desc "Prayer times and the adhan, in your terminal"
  homepage "https://github.com/dimasyotama/adzan-cli"
  version "0.1.0"

  on_macos do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-darwin-arm64.tar.gz"
      sha256 "3ff23a711405772c09e8adf83d2135b702f34d464f2a83edfca6eeb31d7e056d"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-darwin-amd64.tar.gz"
      sha256 "b25499b15b9e1b15f64dc08482e6389b8c0a26fdd4395e6cf4537f054ae5e1e5"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-linux-arm64.tar.gz"
      sha256 "8b7a49f7395844ec22fa691b5d53d9f8b22832a840c822e7da74147e9a72c6e3"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-linux-amd64.tar.gz"
      sha256 "e8b397cd7c4377eca459e588e05a58a1c78f9704afef90d20f12d788143640bf"
    end
  end

  def install
    bin.install "adzan"
    bin.install "adzand"
    bin.install "adzantray" if File.exist?("adzantray")
  end

  service do
    run [opt_bin/"adzand"]
    keep_alive successful_exit: false
    log_path var/"log/adzan.log"
    error_log_path var/"log/adzan.log"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/adzan version")
  end
end
