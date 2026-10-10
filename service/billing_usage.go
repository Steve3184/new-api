package service

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// zeroOutputTokenInsuranceApplies only covers generation requests. Embeddings,
// standalone search, images and other task APIs can succeed without output tokens.
// Inspect canonical totals before expression pricing subtracts output categories.
func zeroOutputTokenInsuranceApplies(info *relaycommon.RelayInfo, usage *dto.Usage) bool {
	if !operation_setting.GetQuotaSetting().ZeroOutputTokenInsurance {
		return false
	}
	if info.RelayFormat != types.RelayFormatClaude {
		switch info.RelayMode {
		case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeCompletions,
			relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact,
			relayconstant.RelayModeSystemOne, relayconstant.RelayModeEdits:
		case relayconstant.RelayModeGemini:
			if strings.Contains(strings.ToLower(info.RequestURLPath), "embed") {
				return false
			}
		default:
			return false
		}
	}
	usage = effectiveBillingUsage(usage)
	if usage == nil {
		return true
	}
	return usage.CompletionTokens == 0 && usage.OutputTokens == 0 &&
		usage.CompletionTokenDetails == (dto.OutputTokenDetails{})
}

const (
	usageBillingPathLocal              = "local"
	usageBillingPathUpstream           = "upstream"
	usageBillingPathOpenAI             = "billing-usage-openai"
	usageBillingPathOpenAIEstimated    = "billing-usage-openai-estimated"
	usageBillingPathAnthropic          = "billing-usage-anthropic"
	usageBillingPathAnthropicEstimated = "billing-usage-anthropic-estimated"
	usageBillingPathGemini             = "billing-usage-gemini"
	usageBillingPathGeminiEstimated    = "billing-usage-gemini-estimated"
)

func effectiveBillingUsage(usage *dto.Usage) *dto.Usage {
	if billingUsage, ok := usageFromBillingUsage(usage); ok {
		return billingUsage
	}
	return usage
}

func usageBillingPathForLog(isLocalCountTokens bool, usage *dto.Usage) string {
	effectiveUsage, ok := usageFromBillingUsage(usage)
	if !ok {
		if isLocalCountTokens {
			return usageBillingPathLocal
		}
		return usageBillingPathUpstream
	}

	switch effectiveUsage.UsageSemantic {
	case dto.BillingUsageSemanticOpenAI:
		if usage.BillingUsage.Estimated {
			return usageBillingPathOpenAIEstimated
		}
		return usageBillingPathOpenAI
	case dto.BillingUsageSemanticAnthropic:
		if usage.BillingUsage.Estimated {
			return usageBillingPathAnthropicEstimated
		}
		return usageBillingPathAnthropic
	case dto.BillingUsageSemanticGemini:
		if usage.BillingUsage.Estimated {
			return usageBillingPathGeminiEstimated
		}
		return usageBillingPathGemini
	}

	return usageBillingPathUpstream
}

func appendUsageBillingPathForLog(other *model.LogOther, isLocalCountTokens bool, usage *dto.Usage) {
	if other == nil {
		return
	}
	other.SetAdmin("usage_billing_path", usageBillingPathForLog(isLocalCountTokens, usage))
}

func usageFromBillingUsage(usage *dto.Usage) (*dto.Usage, bool) {
	if usage == nil || usage.BillingUsage == nil {
		return nil, false
	}
	return usage.BillingUsage.CanonicalUsage()
}
