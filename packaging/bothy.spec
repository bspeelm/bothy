# bothy — RPM packaging for Copr.
#
# This builds from the GitHub source tag rather than repackaging the release
# binary, because a distro package that ships someone else's prebuilt binary is
# a distro package in name only. Copr build roots have no network, so the one
# dependency is vendored in the repository and the build runs with -mod=vendor
# and GOPROXY=off — if either were wrong the build would fail here rather than
# silently reach out.
#
# Note this installs the bothy *binary* system-wide. The workspace it manages
# stays per-user in ~/.local/share/bothy, exactly as it does for any other
# install method; nothing here changes what bothy does at runtime.

%global goipath github.com/bspeelm/bothy
%global debug_package %{nil}

Name:           bothy
Version:        0.5.0
Release:        1%{?dist}
Summary:        A turn-key terminal workspace built from tools you already trust

License:        MIT
URL:            https://github.com/bspeelm/bothy
Source0:        %{url}/archive/refs/tags/v%{version}.tar.gz#/%{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24
BuildRequires:  git-core

# Deliberately no Requires. bothy supplies whatever tool is missing into its own
# directory at first run, checksum-verified, and never asks a package manager
# for anything — declaring dependencies here would be a second, contradictory
# opinion about how the workspace gets its tools. See PLAN.md §4.

%description
bothy launches a persistent terminal layout — a file browser, an agent and a
shell — and tells you what is broken and how to fix it. It installs any tool
you are missing into its own directory, writes its own configs there, and
leaves your dotfiles alone: everything it manages lives under
~/.local/share/bothy, and `bothy uninstall` removes that one directory.

%prep
%autosetup -n %{name}-%{version}

%build
export CGO_ENABLED=0
export GOFLAGS="-mod=vendor"
export GOPROXY=off
go build -ldflags "-s -w -X main.Version=%{version}" -o %{name} ./cmd/%{name}

%install
install -Dpm 0755 %{name} %{buildroot}%{_bindir}/%{name}

%check
export GOFLAGS="-mod=vendor"
export GOPROXY=off
# The bootstrap tests start a local HTTP server, which the build root allows;
# nothing here reaches the network.
go test ./...

%files
%license LICENSE
%doc README.md NOTICE
%{_bindir}/%{name}

%changelog
* Wed Sep 02 2026 Bryan Speelman <bryspeelm@pm.me> - 0.5.0-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.5.0

* Wed Sep 02 2026 Bryan Speelman <bryspeelm@pm.me> - 0.4.0-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.4.0

* Wed Sep 02 2026 Bryan Speelman <bryspeelm@pm.me> - 0.3.2-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.3.2

* Tue Sep 01 2026 Bryan Speelman <bryspeelm@pm.me> - 0.3.1-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.3.1

* Tue Sep 01 2026 Bryan Speelman <bryspeelm@pm.me> - 0.3.0-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.3.0

* Tue Sep 01 2026 Bryan Speelman <bryspeelm@pm.me> - 0.2.0-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.2.0

* Mon Aug 31 2026 Bryan Speelman <bryspeelm@pm.me> - 0.1.5-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.1.5

* Mon Aug 31 2026 Bryan Speelman <bryspeelm@pm.me> - 0.1.4-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.1.4

* Mon Aug 31 2026 Bryan Speelman <bryspeelm@pm.me> - 0.1.3-1
- See https://github.com/bspeelm/bothy/releases/tag/v0.1.3

* Mon Aug 31 2026 Bryan Speelman <bryspeelm@pm.me> - 0.1.2-1
- Initial Copr package.
- Reports its real version when built without the release ldflags.
