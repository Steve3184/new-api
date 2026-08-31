package model

import "strings"

// GetMissingModels returns model names that are referenced in the system
func GetMissingModels() ([]string, error) {
	// 1. 获取所有已启用模型（去重）
	models := GetEnabledModels()
	if len(models) == 0 {
		return []string{}, nil
	}

	// 2. 查询已有的元数据模型名
	var existing []string
	if err := DB.Model(&Model{}).Where("model_name IN ?", models).Pluck("model_name", &existing).Error; err != nil {
		return nil, err
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		existingSet[e] = struct{}{}
	}

	// 3. 收集缺失模型
	var missing []string
	for _, name := range models {
		if _, ok := existingSet[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

// DeleteUnusedDescribedModels removes metadata-only models that are not
// referenced by any enabled channel/ability. It returns the number removed.
func DeleteUnusedDescribedModels() (int64, error) {
	used := make(map[string]struct{})
	for _, name := range GetEnabledModels() {
		used[strings.TrimSpace(name)] = struct{}{}
	}
	var candidates []Model
	if err := DB.Where("description <> '' AND description IS NOT NULL").Find(&candidates).Error; err != nil {
		return 0, err
	}
	var deleted int64
	for _, item := range candidates {
		if _, ok := used[item.ModelName]; ok {
			continue
		}
		result := DB.Delete(&Model{}, item.Id)
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	return deleted, nil
}
