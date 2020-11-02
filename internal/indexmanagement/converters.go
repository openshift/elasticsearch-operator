package indexmanagement

import (
	"fmt"
	"strconv"

	"github.com/ViaQ/logerr/kverrors"
	apis "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
)

func calculateConditions(policy apis.IndexManagementPolicySpec, primaryShards int32) rolloverConditions {
	// 40GB = 40960 1K messages
	maxDoc := constants.TheoreticalShardMaxSizeInMB * 1000 * primaryShards
	maxSize := defaultShardSize * primaryShards
	maxAge := ""
	if policy.Phases.Hot != nil && policy.Phases.Hot.Actions.Rollover != nil {
		maxAge = string(policy.Phases.Hot.Actions.Rollover.MaxAge)
	}
	return rolloverConditions{
		MaxSize: fmt.Sprintf("%dgb", maxSize),
		MaxDocs: maxDoc,
		MaxAge:  maxAge,
	}
}

func calculateMillisForTimeUnit(timeunit apis.TimeUnit) (uint64, error) {
	match := reTimeUnit.FindStringSubmatch(string(timeunit))
	if match == nil || len(match) < 2 {
		return 0, kverrors.New("unable to convert timeunit to millis for invalid timeunit",
			"unit", timeunit)
	}
	n := match[1]
	number, err := strconv.ParseUint(n, 10, 0)
	if err != nil {
		return 0, kverrors.Wrap(err, "unable to parse uint", "value", n)
	}
	switch match[2] {
	case "w":
		return number * millisPerWeek, nil
	case "d":
		return number * millisPerDay, nil
	case "h", "H":
		return number * millisPerHour, nil
	case "m":
		return number * millisPerMinute, nil
	case "s":
		return number * millisPerSecond, nil
	}
	return 0, kverrors.New("conversion to millis for time unit is unsupported", "timeunit", match[2])
}

func crontabScheduleFor(timeunit apis.TimeUnit) (string, error) {
	match := reTimeUnit.FindStringSubmatch(string(timeunit))
	if match == nil {
		return "", kverrors.New("Unable to create crontab schedule for invalid timeunit", "timeunit", timeunit)
	}
	switch match[2] {
	case "m":
		return fmt.Sprintf("*/%s * * * *", match[1]), nil
	}

	return "", kverrors.New("crontab schedule for time unit is unsupported", "timeunit", match[2])
}
