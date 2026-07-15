package playground_setting

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

type Feature string

const (
	FeatureChat   Feature = "chat"
	FeatureImage  Feature = "image"
	FeatureSpeech Feature = "speech"
	FeatureThreeD Feature = "three_d"
)

const (
	SpeechModelTypeOpenAI = "openai"
	SpeechModelTypeAzure  = "azure"
	SpeechModelTypeUnreal = "unrealspeech"
)

type ModelAllowlist struct {
	Chat   []string `json:"chat"`
	Image  []string `json:"image"`
	Speech []string `json:"speech"`
	ThreeD []string `json:"three_d"`
}

type Settings struct {
	EnabledFeatures  []Feature         `json:"enabled_features"`
	Models           ModelAllowlist    `json:"models"`
	SpeechModelTypes map[string]string `json:"speech_model_types"`
}

var (
	settingsMu sync.RWMutex
	settings   = defaultSettings()
)

func defaultSettings() Settings {
	return Settings{
		EnabledFeatures: []Feature{FeatureChat},
		Models: ModelAllowlist{
			Chat:   []string{},
			Image:  []string{},
			Speech: []string{},
			ThreeD: []string{},
		},
		SpeechModelTypes: map[string]string{},
	}
}

func Get() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return cloneSettings(settings)
}

func ToJSONString() string {
	data, err := common.Marshal(Get())
	if err != nil {
		return `{"enabled_features":["chat"],"models":{"chat":[],"image":[],"speech":[],"three_d":[]},"speech_model_types":{}}`
	}
	return string(data)
}

func UpdateByJSONString(value string) error {
	next, err := parseJSONString(value)
	if err != nil {
		return err
	}

	settingsMu.Lock()
	settings = next
	settingsMu.Unlock()
	return nil
}

func ValidateJSONString(value string) error {
	_, err := parseJSONString(value)
	return err
}

func parseJSONString(value string) (Settings, error) {
	next := defaultSettings()
	if err := common.UnmarshalJsonStr(value, &next); err != nil {
		return Settings{}, fmt.Errorf("invalid playground settings: %w", err)
	}
	if err := normalize(&next); err != nil {
		return Settings{}, err
	}
	return next, nil
}

func IsFeatureEnabled(feature Feature) bool {
	current := Get()
	for _, enabled := range current.EnabledFeatures {
		if enabled == feature {
			return true
		}
	}
	return false
}

func IsModelAllowed(feature Feature, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}

	current := Get()
	var allowed []string
	switch feature {
	case FeatureChat:
		allowed = current.Models.Chat
	case FeatureImage:
		allowed = current.Models.Image
	case FeatureSpeech:
		allowed = current.Models.Speech
	case FeatureThreeD:
		allowed = current.Models.ThreeD
	default:
		return false
	}
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == modelName {
			return true
		}
	}
	return false
}

func GetSpeechModelType(modelName string) string {
	current := Get()
	if modelType := current.SpeechModelTypes[strings.TrimSpace(modelName)]; modelType != "" {
		return modelType
	}
	return SpeechModelTypeOpenAI
}

func normalize(value *Settings) error {
	knownFeatures := map[Feature]struct{}{
		FeatureChat: {}, FeatureImage: {}, FeatureSpeech: {}, FeatureThreeD: {},
	}
	features := make([]Feature, 0, len(value.EnabledFeatures))
	seenFeatures := make(map[Feature]struct{}, len(value.EnabledFeatures))
	for _, feature := range value.EnabledFeatures {
		if _, ok := knownFeatures[feature]; !ok {
			return fmt.Errorf("unsupported playground feature: %s", feature)
		}
		if _, exists := seenFeatures[feature]; exists {
			continue
		}
		seenFeatures[feature] = struct{}{}
		features = append(features, feature)
	}
	if len(features) == 0 {
		return errors.New("at least one playground feature must be enabled")
	}
	value.EnabledFeatures = features
	value.Models.Chat = normalizeModels(value.Models.Chat)
	value.Models.Image = normalizeModels(value.Models.Image)
	value.Models.Speech = normalizeModels(value.Models.Speech)
	value.Models.ThreeD = normalizeModels(value.Models.ThreeD)

	modelTypes := make(map[string]string, len(value.SpeechModelTypes))
	for modelName, modelType := range value.SpeechModelTypes {
		modelName = strings.TrimSpace(modelName)
		modelType = strings.ToLower(strings.TrimSpace(modelType))
		if modelName == "" {
			continue
		}
		switch modelType {
		case "", SpeechModelTypeOpenAI:
			modelTypes[modelName] = SpeechModelTypeOpenAI
		case SpeechModelTypeAzure:
			modelTypes[modelName] = SpeechModelTypeAzure
		case SpeechModelTypeUnreal:
			modelTypes[modelName] = SpeechModelTypeUnreal
		default:
			return fmt.Errorf("unsupported speech model type %q for model %s", modelType, modelName)
		}
	}
	value.SpeechModelTypes = modelTypes
	return nil
}

func normalizeModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, exists := seen[modelName]; exists {
			continue
		}
		seen[modelName] = struct{}{}
		result = append(result, modelName)
	}
	sort.Strings(result)
	return result
}

func cloneSettings(source Settings) Settings {
	result := source
	result.EnabledFeatures = append([]Feature(nil), source.EnabledFeatures...)
	result.Models.Chat = append([]string(nil), source.Models.Chat...)
	result.Models.Image = append([]string(nil), source.Models.Image...)
	result.Models.Speech = append([]string(nil), source.Models.Speech...)
	result.Models.ThreeD = append([]string(nil), source.Models.ThreeD...)
	result.SpeechModelTypes = make(map[string]string, len(source.SpeechModelTypes))
	for modelName, modelType := range source.SpeechModelTypes {
		result.SpeechModelTypes[modelName] = modelType
	}
	return result
}
