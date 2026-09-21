class MiniMe < Formula
  desc "Local-first, single-user personal knowledge base for macOS"
  homepage "https://github.com/bogusdeck/mini-me"
  url "https://github.com/bogusdeck/mini-me/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "da9d4914fc9333a6b097df87eb8d58c4d16c42a91a1d5512ec01975a6a36fa0c"
  head "https://github.com/bogusdeck/mini-me.git", branch: "main"
  version "0.1.0"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/mini-me"
  end

  service do
    run [opt_bin/"mini-me", "serve", "--host", "127.0.0.1", "--port", "8080"]
    keep_alive true
    working_dir var
    log_path var/"log/mini-me.log"
    error_log_path var/"log/mini-me.error.log"
  end

  test do
    assert_match "mini-me v#{version}", shell_output("#{bin}/mini-me version")
  end
end
