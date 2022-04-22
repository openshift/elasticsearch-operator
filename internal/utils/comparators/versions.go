package comparators

import (
	"strconv"
	"strings"
)

type Version string

func (v Version) ToArray() ([]int, error) {
	sections := strings.Split(string(v), ".")
	versionNumbers := make([]int, len(sections))

	for i, s := range sections {
		vn, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		versionNumbers[i] = vn
	}

	return versionNumbers, nil
}

// CompareVersionArrays will return one of:
// -1 : if lhs > rhs
// 0  : if lhs == rhs
// 1  : if rhs > lhs
func CompareVersionArrays(lhs, rhs []int) int {
	lLen := len(lhs)
	rLen := len(rhs)

	for i := 0; i < lLen && i < rLen; i++ {
		if lhs[i] > rhs[i] {
			return -1
		}

		if lhs[i] < rhs[i] {
			return 1
		}
	}

	// check if lhs is a more specific version number (aka newer)
	if lLen > rLen {
		return -1
	}

	// check if rhs is a more specific version number
	if lLen < rLen {
		return 1
	}

	// versions are exactly the same
	return 0
}
