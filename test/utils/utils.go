package utils

import (
	"strings"
	"time"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
)

const (
	DefaultRetryInterval        = time.Second * 3
	DefaultTimeout              = time.Second * 300
	DefaultCleanupRetryInterval = time.Second * 3
	DefaultCleanupTimeout       = time.Second * 30
)

func GenerateUUID() string {
	uuid, err := utils.RandStringBytes(8)
	if err != nil {
		return ""
	}

	return uuid
}

func parseString(path string, interfaceMap map[string]interface{}) string {
	value := walkInterfaceMap(path, interfaceMap)

	if parsedString, ok := value.(string); ok {
		return parsedString
	} else {
		return ""
	}
}

func walkInterfaceMap(path string, interfaceMap map[string]interface{}) interface{} {

	current := interfaceMap
	keys := strings.Split(path, ".")
	keyCount := len(keys)

	for index, key := range keys {
		if current[key] != nil {
			if index+1 < keyCount {
				current = current[key].(map[string]interface{})
			} else {
				return current[key]
			}
		} else {
			return nil
		}
	}

	return nil
}
