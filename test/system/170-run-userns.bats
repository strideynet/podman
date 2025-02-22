#!/usr/bin/env bats   -*- bats -*-
# shellcheck disable=SC2096
#
# Tests for podman build
#

load helpers

function _require_crun() {
    runtime=$(podman_runtime)
    if [[ $runtime != "crun" ]]; then
        skip "runtime is $runtime; keep-groups requires crun"
    fi
}

@test "podman --group-add keep-groups while in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    _require_crun
    run chroot --groups 1234 / ${PODMAN} run --rm --uidmap 0:200000:5000 --group-add keep-groups $IMAGE id
    is "$output" ".*65534(nobody)" "Check group leaked into user namespace"
}

@test "podman --group-add keep-groups while not in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    _require_crun
    run chroot --groups 1234,5678 / ${PODMAN} run --rm --group-add keep-groups $IMAGE id
    is "$output" ".*1234" "Check group leaked into container"
}

@test "podman --group-add without keep-groups while in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    run chroot --groups 1234,5678 / ${PODMAN} run --rm --uidmap 0:200000:5000 --group-add 457 $IMAGE id
    is "$output" ".*457" "Check group leaked into container"
}

@test "podman --remote --group-add keep-groups " {
    if is_remote; then
        run_podman 125 run --rm --group-add keep-groups $IMAGE id
        is "$output" ".*not supported in remote mode" "Remote check --group-add keep-groups"
    fi
}

@test "podman --group-add without keep-groups " {
    run_podman run --rm --group-add 457 $IMAGE id
    is "$output" ".*457" "Check group leaked into container"
}

@test "podman --group-add keep-groups plus added groups " {
    run_podman 125 run --rm --group-add keep-groups --group-add 457 $IMAGE id
    is "$output" ".*the '--group-add keep-groups' option is not allowed with any other --group-add options" "Check group leaked into container"
}

@test "podman userns=auto in config file" {
    skip_if_remote "userns=auto is set on the server"

    if is_rootless; then
        egrep -q "^$(id -un):" /etc/subuid || skip "no IDs allocated for current user"
    else
        egrep -q "^containers:" /etc/subuid || skip "no IDs allocated for user 'containers'"
    fi

    cat > $PODMAN_TMPDIR/userns_auto.conf <<EOF
[containers]
userns="auto"
EOF
    # First make sure a user namespace is created
    CONTAINERS_CONF=$PODMAN_TMPDIR/userns_auto.conf run_podman run -d $IMAGE sleep infinity
    cid=$output

    run_podman inspect --format '{{.HostConfig.UsernsMode}}' $cid
    is "$output" "private" "Check that a user namespace was created for the container"

    run_podman rm -t 0 -f $cid

    # Then check that the main user is not mapped into the user namespace
    CONTAINERS_CONF=$PODMAN_TMPDIR/userns_auto.conf run_podman 0 run --rm $IMAGE awk '{if($2 == "0"){exit 1}}' /proc/self/uid_map /proc/self/gid_map
}
