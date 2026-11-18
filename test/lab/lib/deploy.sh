# shellcheck shell=bash
# The package under test, and the acceptance run.

# package_path: the .deb under test (LAB_DEB, or the newest in dist/).
package_path() {
	if [[ -n "$LAB_DEB" ]]; then
		echo "$LAB_DEB"
		return
	fi
	local deb
	deb="$(ls -t "$REPO_DIR"/dist/netcheck_*_linux_amd64.deb 2>/dev/null | head -1 || true)"
	[[ -n "$deb" ]] || die "no .deb in dist/: run make snapshot, or set LAB_DEB"
	echo "$deb"
}

# deploy: installs the package on nc-client with dpkg, as a user would, and
# records its checksum (LT-5).
deploy() {
	section "Package"
	local deb sum
	deb="$(package_path)"
	sum="$(sha256sum "$deb" | cut -d' ' -f1)"
	info "installing $(basename "$deb") ($sum)"
	lxd file push "$deb" nc-client/root/netcheck.deb >/dev/null
	vm_run nc-client dpkg -i /root/netcheck.deb
	echo "$sum  $(basename "$deb")" >"$LAB_STATE_DIR/package.sha256"
}

# run_tests [GO-FLAGS]: the acceptance scenarios, which drive the lab over lxc.
run_tests() {
	section "Acceptance"
	mkdir -p "$LAB_STATE_DIR"
	(
		cd "$REPO_DIR"
		NETCHECK_LAB_PROJECT="$LAB_PROJECT" \
			NETCHECK_LAB_SERVERS_IP="$(vm_ip nc-servers servers)" \
			NETCHECK_LAB_STATE="$LAB_STATE_DIR" \
			go test -tags lab -count=1 -timeout 30m -v "$@" ./test/lab/acceptance
	)
}
