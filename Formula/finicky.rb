# Homebrew formula for this fork of Finicky. It builds the app from the
# repository's main branch, so `brew upgrade` follows git.
#
#   brew tap hanslemm/finicky https://github.com/hanslemm/finicky.git
#   brew install --HEAD hanslemm/finicky/finicky
#   brew upgrade --fetch-HEAD hanslemm/finicky/finicky
#
# See docs/manual/chapters/build-and-version.md.
class Finicky < Formula
  desc "Always open the right browser (fork of johnste/finicky)"
  homepage "https://github.com/hanslemm/finicky"
  license "MIT"
  head "https://github.com/hanslemm/finicky.git", branch: "main"

  depends_on "go" => :build
  depends_on "node" => :build
  depends_on :macos

  def install
    arch = Hardware::CPU.arm? ? "arm64" : "amd64"
    ENV["BUILD_TARGET_ARCH"] = arch
    ENV["COMMIT_HASH"] = version.to_s.delete_prefix("HEAD-")

    cd("packages/config-api") { system "npm", "ci", "--no-audit", "--no-fund" }
    cd("packages/finicky-ui") { system "npm", "ci", "--no-audit", "--no-fund" }
    system "./scripts/build.sh"

    app = "apps/finicky/build/Finicky-#{arch}.app"
    # Sign the whole bundle (ad hoc) so macOS sees the bundle id, not "a.out".
    system "codesign", "--force", "--deep", "--sign", "-", app
    prefix.install app => "Finicky.app"
  end

  def post_install
    # Tell Launch Services where the new build is, so links reach it.
    lsregister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/" \
                 "LaunchServices.framework/Support/lsregister"
    quiet_system lsregister, "-f", "#{opt_prefix}/Finicky.app"
  end

  def caveats
    <<~EOS
      Finicky.app is in #{opt_prefix}. Link it into /Applications once:
        ln -sfn #{opt_prefix}/Finicky.app /Applications/Finicky.app

      After an upgrade, quit Finicky (menu bar icon > Quit). The next link starts the new build.
    EOS
  end

  test do
    assert_path_exists prefix/"Finicky.app/Contents/MacOS/Finicky"
    assert_equal "se.johnste.finicky",
                 shell_output("/usr/bin/defaults read #{prefix}/Finicky.app/Contents/Info.plist CFBundleIdentifier").strip
  end
end
