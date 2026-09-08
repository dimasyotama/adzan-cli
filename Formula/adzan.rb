class Adzan < Formula
  desc "Prayer times and the adhan, in your terminal"
  homepage "https://github.com/dimasyotama/adzan-cli"
  version "0.1.1"

  on_macos do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.1/adzan-0.1.1-darwin-arm64.tar.gz"
      sha256 "cdb5a5d46b16d69ea08afa55a34de6e2c2b7a83e16738aba5b8722ea3cd981e7"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.1/adzan-0.1.1-darwin-amd64.tar.gz"
      sha256 "20c43983d764fd8ac4f5e3a4c9b19915831f2d034135e4706a97bc624a11de7d"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.1/adzan-0.1.1-linux-arm64.tar.gz"
      sha256 "6b38950366b3955df5a63468a335a776538b8a50cb8a58e1db2a94013219bb48"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.1/adzan-0.1.1-linux-amd64.tar.gz"
      sha256 "b422559a099aa923ac6a83cbe0a2c3371ce8315df08f34680dadf5bbdde976d8"
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
