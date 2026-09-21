# PKGBUILD for mini-me (Arch Linux / pacman)
pkgname=mini-me
pkgver=0.1.0
pkgrel=1
pkgdesc="Local-first personal digital twin and RAG context engine"
arch=('x86_64' 'aarch64')
url="https://github.com/bogusdeck/mini-me"
license=('MIT')
depends=('glibc')
makedepends=('go')
source=("$pkgname-$pkgver.tar.gz::$url/archive/refs/tags/v$pkgver.tar.gz")
sha256sums=('da9d4914fc9333a6b097df87eb8d58c4d16c42a91a1d5512ec01975a6a36fa0c')

build() {
  cd "$pkgname-$pkgver"
  export CGO_ENABLED=0
  go build \
    -trimpath \
    -buildmode=pie \
    -mod=readonly \
    -ldflags "-s -w -X main.version=v$pkgver" \
    -o mini-me ./cmd/mini-me
}

package() {
  cd "$pkgname-$pkgver"
  install -Dm755 mini-me "$pkgdir/usr/bin/mini-me"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}
