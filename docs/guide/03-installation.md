# Installation

## From a package

Each release publishes packages for Linux, for `amd64` and `arm64`: `.deb`, `.rpm` and `.apk`. A package installs:

- the binary `netcheck` in `/usr/bin`;
- the defaults file `/etc/netcheck.conf` (a configuration file: an upgrade keeps your changes);
- a post-install step that lets the binary send ICMP echo requests without being root (see below).

```bash
$> sudo dpkg -i netcheck_<version>_linux_amd64.deb      # Debian, Ubuntu
$> sudo rpm -i netcheck_<version>_linux_amd64.rpm       # Red Hat, Fedora, SUSE
$> sudo apk add --allow-untrusted netcheck_<version>_linux_amd64.apk   # Alpine
```

On x86-64, the release has the plain `amd64` build, which runs on any CPU, and the `amd64v2` and `amd64v3` builds, tuned for newer CPUs. If in doubt, use the plain one.

## From an archive

Archives for Linux, macOS (Darwin) and Windows, for `amd64` and `arm64`, hold the binary alone. Unpack the archive and put the binary on your `PATH`.

## Verifying a download

Every release has a file `netcheck_<version>_checksums.txt` that lists the SHA-256 of every archive, package and document, and its detached GPG signature `netcheck_<version>_checksums.txt.asc`. The public key is `packaging/signing-key.asc` in the source repository.

```bash
$> gpg --import signing-key.asc
$> gpg --verify netcheck_<version>_checksums.txt.asc netcheck_<version>_checksums.txt
$> sha256sum --check --ignore-missing netcheck_<version>_checksums.txt
```

The `.deb` and `.rpm` packages are also signed with the same key. Each package has a software bill of materials (`.cdx.json`, CycloneDX) next to it.

## ICMP checks need a privilege

An ICMP check sends raw packets. On Linux the packages give the binary the capability `cap_net_raw` in their post-install step. With an archive, or when the step failed, do it yourself:

```bash
$> sudo setcap cap_net_raw=+ep /usr/bin/netcheck
```

Without it, ICMP checks fail with a permission error; all the other protocols work as an ordinary user. On Windows the checks use the system's ICMP service and need no extra step.

## The defaults file

`netcheck` looks for `netcheck.conf`, a YAML file, in these places and uses the first one it finds (the files are not merged):

| System | Search order |
| -------------- | -------------------------------------------------------- |
| Linux, macOS | `./netcheck.conf`, `~/netcheck.conf`, `/etc/netcheck.conf` |
| Windows | `./netcheck.conf`, `~/netcheck.conf` |

A value missing from the file takes the compiled-in default (chapter 2). The file the package installs is also `examples/netcheck.conf` in the source:

```{.yaml include="examples/netcheck.conf"}
```

## Building from source

You need Go, `make`, GoReleaser and golangci-lint. `make compile` checks the code and builds `dist/netcheck_linux_amd64_v1/netcheck`; `make help` lists every target.
