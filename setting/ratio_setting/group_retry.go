package ratio_setting

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const MaxGroupRetryTimes = 10

var groupRetryTimesMap = types.NewRWMap[string, int]()

func GroupRetryTimes2JSONString() string {
	return groupRetryTimesMap.MarshalJSONString()
}

func UpdateGroupRetryTimesByJSONString(jsonStr string) error {
	if err := CheckGroupRetryTimes(jsonStr); err != nil {
		return err
	}
	return types.LoadFromJsonString(groupRetryTimesMap, jsonStr)
}

func GetGroupRetryTimes(group string, fallback int) int {
	retryTimes, ok := groupRetryTimesMap.Get(strings.TrimSpace(group))
	if !ok {
		return fallback
	}
	return retryTimes
}

func CheckGroupRetryTimes(jsonStr string) error {
	retryTimes := make(map[string]int)
	if err := common.UnmarshalJsonStr(jsonStr, &retryTimes); err != nil {
		return err
	}
	for group, count := range retryTimes {
		if strings.TrimSpace(group) == "" {
			return errors.New("group name must not be empty")
		}
		if count < 0 || count > MaxGroupRetryTimes {
			return fmt.Errorf("retry times for group %s must be between 0 and %d", group, MaxGroupRetryTimes)
		}
	}
	return nil
}
