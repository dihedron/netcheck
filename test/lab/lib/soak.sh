# shellcheck shell=bash
# The soak (LB-07): hours of the real command against the real servers with
# seeded faults, and the race-enabled stress binary running beside it.

# soak [--hours H | --seconds S] [--now] [--yes]: refuses to start in the
# working day without --now (AGENTS.md rule 14, LT-4).
soak() {
	local seconds=$((LAB_SOAK_HOURS * 3600)) now=no
	while (($# > 0)); do
		case "$1" in
		--hours) seconds=$(($2 * 3600)) && shift ;;
		--seconds) seconds="$2" && shift ;;
		--now) now=yes ;;
		--yes) ASSUME_YES=yes ;;
		*) die "soak: unknown option $1" ;;
		esac
		shift
	done
	local hour
	hour="$(date +%-H)"
	if [[ "$now" != yes ]] && ((hour >= LAB_DAY_START && hour < LAB_DAY_END)); then
		die "it is $(date +%H:%M): a soak runs overnight (outside $LAB_DAY_START:00-$LAB_DAY_END:00); pass --now to run it anyway"
	fi
	confirm "Start a soak of $((seconds / 60)) minutes on the lab?" || die "not confirmed"
	section "Soak: $((seconds / 60)) minutes"
	local race="$LAB_STATE_DIR/netcheck.stress.race" plain="$LAB_STATE_DIR/netcheck.stress.plain"
	(cd "$REPO_DIR" && go test -c -race -tags stress -o "$race" ./pkg/netcheck && go test -c -tags stress -o "$plain" ./pkg/netcheck)
	vm_run nc-client sh -c 'pkill -f "[n]etcheck.stress"; sleep 1; true'
	vm_push nc-client "$race" /root/netcheck.stress.race
	vm_push nc-client "$plain" /root/netcheck.stress.plain
	vm_run nc-client chmod +x /root/netcheck.stress.race /root/netcheck.stress.plain
	(
		cd "$REPO_DIR"
		NETCHECK_LAB_PROJECT="$LAB_PROJECT" NETCHECK_LAB_SERVERS_IP="$(vm_ip nc-servers servers)" \
			NETCHECK_LAB_STATE="$LAB_STATE_DIR" NETCHECK_SOAK_SECONDS="$seconds" \
			go test -tags lab -count=1 -timeout 0 -v -run 'TestSoak$' ./test/lab/acceptance
	)
}
