package indexmanagement

import (
	"fmt"
	"strconv"

	apis "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/constants"
	"github.com/openshift/elasticsearch-operator/pkg/log"
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
		return 0, fmt.Errorf("Unable to convert timeunit to millis for invalid timeunit %q", timeunit)
	}
	n := match[1]
	number, err := strconv.ParseUint(n, 10, 0)
	if err != nil {
		log.Error(err, "unable to parse uint", "raw", n)
		return 0, err
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
	return 0, fmt.Errorf("conversion to millis for time unit %q is unsupported", match[2])
}

func crontabScheduleFor(timeunit apis.TimeUnit) (string, error) {
	match := reTimeUnit.FindStringSubmatch(string(timeunit))
	if match == nil {
		return "", fmt.Errorf("Unable to create crontab schedule for invalid timeunit %q", timeunit)
	}
	switch match[2] {
	case "m":
		return fmt.Sprintf("*/%s * * * *", match[1]), nil
	}

	return "", fmt.Errorf("crontab schedule for time unit %q is unsupported", match[2])
}
