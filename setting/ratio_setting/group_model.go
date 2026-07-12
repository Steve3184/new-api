package ratio_setting

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

var groupDefaultModelMap = types.NewRWMap[string, string]()

func GroupDefaultModel2JSONString() string {
	return groupDefaultModelMap.MarshalJSONString()
}

func UpdateGroupDefaultModelByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupDefaultModelMap, jsonStr)
}

func GetGroupDefaultModel(group string) string {
	model, _ := groupDefaultModelMap.Get(group)
	return strings.TrimSpace(model)
}

func CheckGroupModelMap(jsonStr string) error {
	models := make(map[string]string)
	if err := common.Unmarshal([]byte(jsonStr), &models); err != nil {
		return err
	}
	for group, model := range models {
		if strings.TrimSpace(group) == "" {
			return errors.New("group name must not be empty")
		}
		if strings.TrimSpace(model) == "" {
			return errors.New("model name must not be empty for group: " + group)
		}
	}
	return nil
}
