#!/bin/bash

set -euo pipefail

current_dir=$(dirname "${BASH_SOURCE[0]}" )
source "${current_dir}/lib/init.sh"

source "${current_dir}/../.bingo/variables.env"

export GO_JUNIT_REPORT="${GO_JUNIT_REPORT:-go-junit-report}"
export JUNITREPORT="${JUNITREPORT:-junitreport}"
export JUNIT_REPORT_OUTPUT="${JUNIT_REPORT_OUTPUT_DIR:-/tmp/artifacts/junit}/junit.out"

EXCLUDES=" "
for test in $( find "${current_dir}/testing-olm" -type f -name 'test-*.sh' | sort); do
	if [[ ${test} =~ .*${EXCLUDES}.* ]] ; then
		os::log::info "==============================================================="
		os::log::info "skipping e2e that was excluded $test "
		os::log::info "==============================================================="
		continue
	fi
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

ARTIFACT_DIR="$JUNIT_REPORT_OUTPUT_DIR" os::test::junit::generate_report

if [[ -n "${failed:-}" ]]; then
    exit 1
fi
