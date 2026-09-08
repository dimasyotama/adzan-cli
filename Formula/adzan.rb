class Adzan < Formula
  desc "Prayer times and the adhan, in your terminal"
  homepage "https://github.com/dimasyotama/adzan-cli"
  version "0.1.3"

  on_macos do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.3/adzan-0.1.3-darwin-arm64.tar.gz"
      sha256 "080e47f0d2fe024fab22863d0f30efebd16c3e465cc0ba1fb36777e7d1706431"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.3/adzan-0.1.3-darwin-amd64.tar.gz"
      sha256 "43b5b5b632227d8e888897775f2ac98c17037382b140ba47fc97e7dd7394a784"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.3/adzan-0.1.3-linux-arm64.tar.gz"
      sha256 "cfb5853626db213c19814617f42283dee2b7c460cc6260727ae5638171d5d460"
    end
    on_intel do
      url "https://github.com/dimasyotama/adzan-cli/releases/download/v0.1.3/adzan-0.1.3-linux-amd64.tar.gz"
      sha256 "5884690af41fcbc97aa807c5c925f866ae697ec0d66890de536861e3f5acde25"
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
