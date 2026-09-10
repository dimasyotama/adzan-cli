class Adzan < Formula
  desc "Prayer times and the adhan, in your terminal"
  homepage "https://github.com/dimasyotama/adzan-cli"
  version "0.1.4"

  on_macos do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.4/adzan-0.1.4-darwin-arm64.tar.gz"
      sha256 "48d6b88df2094b5a13c272106d088d0b6166ee10d511c27c2701fb105fd7dcee"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.4/adzan-0.1.4-darwin-amd64.tar.gz"
      sha256 "16314708f38a1de1be1621412484237fe84661ef76f921c0ffe20753294077d6"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.4/adzan-0.1.4-linux-arm64.tar.gz"
      sha256 "6728cace8e302843305703bc90a723d29077b24ef3a1ba7b2e7950c8f8251656"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.4/adzan-0.1.4-linux-amd64.tar.gz"
      sha256 "585c948668c66d124a705801f0e9ef3bfd6008dbf1dd21d6378b19a9c782ef32"
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
