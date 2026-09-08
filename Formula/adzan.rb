class Adzan < Formula
  desc "Prayer times and the adhan, in your terminal"
  homepage "https://github.com/dimasyotama/adzan-cli"
  version "0.1.2"

  on_macos do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.2/adzan-0.1.2-darwin-arm64.tar.gz"
      sha256 "6f72bda86fb0dd6794f69eb4ed8c07cd5d935a44cd630f608a1d93cdd623dadc"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.2/adzan-0.1.2-darwin-amd64.tar.gz"
      sha256 "661efabe5614613cc43df535ae1e2c5b9518e24cf56a2f7ba3263072a4511b68"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.2/adzan-0.1.2-linux-arm64.tar.gz"
      sha256 "11e15ea2e6d6fec500583d248aeafac53cbad7b347bfe0dbbd963163baee9694"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.2/adzan-0.1.2-linux-amd64.tar.gz"
      sha256 "c1a5afd6b994acfb206e0ec9091080f29f26d83dac6680b69aa288a8698880ce"
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
