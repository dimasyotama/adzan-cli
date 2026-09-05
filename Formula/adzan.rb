class Adzan < Formula
  desc "Prayer times and the adhan, in your terminal"
  homepage "https://github.com/dimasyotama/adzan-cli"
  version "0.1.0"

  on_macos do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-darwin-arm64.tar.gz"
      sha256 "8e6ad2ab4a94d72e19e26dd1440308e50c65f8f3d3cef3e22967e388f2d24a78"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-darwin-amd64.tar.gz"
      sha256 "53a011ca702f6c1c88e2dbfd8092234141b6dd745da4024d7bf68bd2e60c0b41"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-linux-arm64.tar.gz"
      sha256 "d4fd7c6b9ab73fa26da86927e11d8e2a27258417efdaca23120b7cccde15b19b"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.0/adzan-0.1.0-linux-amd64.tar.gz"
      sha256 "057884f3890f3516f429331a15abe4cf3a2003be8f8f5261fe10e4fd212f05aa"
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
