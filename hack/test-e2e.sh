#!/bin/bash

if [ "${DEBUG:-}" = "true" ]; then
  set -x
fi
set -euo pipefail

current_dir=$(dirname "${BASH_SOURCE[0]}" )
source "${current_dir}/lib/init.sh"
source "${current_dir}/lib/util/logs.sh"

# TODO Re-enable test-200-verify-es-metrics-access.sh when es-proxy provides a separate listener
for test in $( find "${current_dir}/testing" -type f -name 'test-001*.sh' | sort); do
	os::log::info "==============================================================="
	os::log::info "running e2e $test "
	os::log::info "==============================================================="
	if "${test}" ; then
		os::log::info "==========================================================="
		os::log::info "e2e $test succeeded at $( date )"
		os::log::info "==========================================================="
	else

		os::log::error "============= FAILED FAILED ============= "
		os::log::error "e2e $test failed at $( date )"
		os::log::error "============= FAILED FAILED ============= "
		failed="true"
	fi
done

get_logging_pod_logs

if [[ -n "${failed:-}" ]]; then
    exit 1
fi
