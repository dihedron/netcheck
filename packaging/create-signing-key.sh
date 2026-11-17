#!/usr/bin/env bash
# Creates the project's GPG packaging key (T-7), publishes its public half as
# packaging/signing-key.asc and stores the private half and its passphrase as
# the GitHub repository secrets the release workflow reads (GPG_PRIVATE_KEY,
# GPG_PASSPHRASE).
#
# Run it once, from a trusted machine, in a clone of the repository:
#
#   packaging/create-signing-key.sh [BACKUP_DIR]
#
# The key is created in a throwaway keyring, never in yours. BACKUP_DIR
# (default: ~/netcheck-packaging-key) receives the private key and its
# revocation certificate: move them to offline storage, they are the only
# copies outside GitHub's secrets.
#
# Needs gpg, gh (logged in, with admin rights on the repository) and git.
# Override the key's identity with KEY_UID, its lifetime with KEY_EXPIRY.
set -euo pipefail

KEY_UID="${KEY_UID:-netcheck packaging <dihedron.dev@gmail.com>}"
# rpm, dnf and debsig-verify on every supported distribution accept RSA
KEY_ALGO=rsa4096
KEY_EXPIRY="${KEY_EXPIRY:-3y}"

die() {
	echo "error: $*" >&2
	exit 1
}

for tool in gpg gh git; do
	command -v "$tool" >/dev/null || die "$tool is not installed"
done
repo="$(git rev-parse --show-toplevel)" || die "run this in a clone of the repository"
public="$repo/packaging/signing-key.asc"
[[ ! -e "$public" ]] || die "$public exists: the key was already created (rotate it on purpose, by removing the file first)"
gh auth status >/dev/null 2>&1 || die "gh is not logged in (gh auth login)"
slug="$(cd "$repo" && gh repo view --json nameWithOwner -q .nameWithOwner)"

backup="${1:-$HOME/netcheck-packaging-key}"
mkdir -p "$backup"
chmod 700 "$backup"
[[ ! -e "$backup/private-key.asc" ]] || die "$backup/private-key.asc exists"

# 1. The passphrase, asked twice, never echoed nor passed on a command line.
read -rsp "Passphrase for the packaging key: " passphrase && echo
read -rsp "Again: " again && echo
[[ "$passphrase" == "$again" ]] || die "the passphrases differ"
[[ ${#passphrase} -ge 16 ]] || die "use at least 16 characters"

# 2. The key, in a throwaway keyring.
GNUPGHOME="$(mktemp -d)"
export GNUPGHOME
trap 'gpgconf --kill gpg-agent 2>/dev/null || true; rm -rf "$GNUPGHOME"' EXIT
gpg() { command gpg --batch --quiet --pinentry-mode loopback --passphrase-fd 3 "$@" 3<<<"$passphrase"; }

echo "creating the $KEY_ALGO key \"$KEY_UID\" (expires in $KEY_EXPIRY)"
gpg --quick-gen-key "$KEY_UID" "$KEY_ALGO" sign "$KEY_EXPIRY"
fpr="$(gpg --with-colons --list-secret-keys | awk -F: '$1 == "fpr" { print $10; exit }')"
[[ -n "$fpr" ]] || die "the key was not created"

# 3. The backups: the private key (still protected by the passphrase) and
#    the revocation certificate gpg made with it.
(umask 077 && gpg --armor --export-secret-keys "$fpr" >"$backup/private-key.asc")
cp "$GNUPGHOME/openpgp-revocs.d/$fpr.rev" "$backup/revocation-certificate.asc"
chmod 600 "$backup/revocation-certificate.asc"

# 4. The public half, in the repository.
gpg --armor --export "$fpr" >"$public"

# 5. The secrets, read by gh from standard input.
echo "setting the secrets GPG_PRIVATE_KEY and GPG_PASSPHRASE on $slug"
gh secret set GPG_PRIVATE_KEY --repo "$slug" <"$backup/private-key.asc"
printf '%s' "$passphrase" | gh secret set GPG_PASSPHRASE --repo "$slug"

cat <<EOF

Done. Fingerprint: $fpr

Next:
  1. Move $backup to offline storage (it holds the private key and its
     revocation certificate) and keep the passphrase in your password manager.
  2. Review and commit the public key:
       git add packaging/signing-key.asc
       git commit -m "Publish the packaging key ($fpr)"
  3. Publish the fingerprint somewhere other than the repository (your
     profile, the project's website), so users can cross-check the key.
  4. Before it expires (in $KEY_EXPIRY), extend it from the backup, in a
     throwaway keyring:
       export GNUPGHOME=\$(mktemp -d)
       gpg --import private-key.asc
       gpg --quick-set-expire $fpr 3y
       gpg --armor --export $fpr > packaging/signing-key.asc
       gpg --armor --export-secret-keys $fpr | gh secret set GPG_PRIVATE_KEY
     then commit packaging/signing-key.asc and replace the backup.
EOF
