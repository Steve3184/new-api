package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterCandidateIDs(t *testing.T) {
	alphaSetting := `{"task_plugin_key":"alpha"}`
	betaSetting := `{"task_plugin_key":"beta"}`
	alpha := &Channel{Id: 900001, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Setting: &alphaSetting}
	beta := &Channel{Id: 900002, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Setting: &betaSetting}
	ordinary := &Channel{Id: 900003, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled}
	kling := &Channel{Id: 900004, Type: constant.ChannelTypeKling, Status: common.ChannelStatusEnabled}
	jimeng := &Channel{Id: 900005, Type: constant.ChannelTypeJimeng, Status: common.ChannelStatusEnabled}
	matchingCustom := &Channel{Id: 900010, Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled}
	matchingCustom.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{{
				IncomingPath: "/v1/chat/completions",
				Models:       []string{"gpt-4"},
			}},
		},
	})
	otherCustom := &Channel{Id: 900011, Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled}
	otherCustom.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{{
				IncomingPath: "/v1/responses",
				Models:       []string{"gpt-4"},
			}},
		},
	})
	typeSafe := &Channel{Id: 900012, Type: constant.ChannelTypeTypeSafe, Status: common.ChannelStatusEnabled}

	pathFilter := dto.ChannelFilter{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}
	systemOnePathFilter := dto.ChannelFilter{Kind: dto.FilterRequestPath, RequestPath: "/v1/systemone"}
	emptyPathFilter := dto.ChannelFilter{Kind: dto.FilterRequestPath, RequestPath: ""}

	tests := []struct {
		name      string
		ids       []int
		modelName string
		filters   []dto.ChannelFilter
		wantKept  []int
		wantEmpty dto.ChannelFilterKind
	}{
		{
			name:      "identity keeps matching task plugin key",
			ids:       []int{900001, 900002},
			modelName: "shared",
			filters:   identityFilters("alpha", nil),
			wantKept:  []int{900001},
		},
		{
			name:      "identity empty key drops all task plugin channels",
			ids:       []int{900001, 900002},
			modelName: "shared",
			filters:   identityFilters("", nil),
			wantKept:  []int{},
			wantEmpty: dto.FilterTaskPluginIdentity,
		},
		{
			name:      "identity empty key keeps ordinary channel",
			ids:       []int{900003},
			modelName: "ordinary",
			filters:   identityFilters("", nil),
			wantKept:  []int{900003},
		},
		{
			name:      "identity keeps matching legacy type",
			ids:       []int{900004, 900005},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", []int{constant.ChannelTypeKling}),
			wantKept:  []int{900004},
		},
		{
			name:      "identity keeps all listed legacy types",
			ids:       []int{900004, 900005},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", []int{constant.ChannelTypeKling, constant.ChannelTypeJimeng}),
			wantKept:  []int{900004, 900005},
		},
		{
			name:      "identity keyed with no types drops legacy",
			ids:       []int{900004, 900005},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", nil),
			wantKept:  []int{},
			wantEmpty: dto.FilterTaskPluginIdentity,
		},
		{
			name:      "identity drops missing cache entry",
			ids:       []int{900004, 999999},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", []int{constant.ChannelTypeKling}),
			wantKept:  []int{900004},
		},
		{
			name:      "empty request path is a passthrough including missing ids",
			ids:       []int{900003, 900010, 999999},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{emptyPathFilter},
			wantKept:  []int{900003, 900010, 999999},
		},
		{
			name:      "request path keeps missing cache entry for consistency",
			ids:       []int{900003, 999999},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter},
			wantKept:  []int{900003, 999999},
		},
		{
			name:      "request path keeps matching type-58 and ordinary",
			ids:       []int{900003, 900010, 900011},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter},
			wantKept:  []int{900003, 900010},
		},
		{
			name:      "system one path selects TypeSafe",
			ids:       []int{900003, 900012},
			modelName: "jev-1.13",
			filters:   []dto.ChannelFilter{systemOnePathFilter},
			wantKept:  []int{900012},
		},
		{
			name:      "ordinary path excludes TypeSafe",
			ids:       []int{900003, 900012},
			modelName: "jev-1.13",
			filters:   []dto.ChannelFilter{pathFilter},
			wantKept:  []int{900003},
		},
		{
			name:      "request path empties when only unmatched type-58 remains",
			ids:       []int{900011},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter},
			wantKept:  []int{},
			wantEmpty: dto.FilterRequestPath,
		},
		{
			name:      "intersection attributes empty set to identity after path keeps candidates",
			ids:       []int{900001, 900010},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter, identityFilters("missing", nil)[0]},
			wantKept:  []int{},
			wantEmpty: dto.FilterTaskPluginIdentity,
		},
		{
			name:      "intersection attributes empty set to path when path runs first",
			ids:       []int{900011},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{identityFilters("", nil)[0], pathFilter},
			wantKept:  []int{},
			wantEmpty: dto.FilterRequestPath,
		},
	}

	channelSyncLock.Lock()
	previous := channelsIDM
	channelsIDM = map[int]*Channel{
		900001: alpha,
		900002: beta,
		900003: ordinary,
		900004: kling,
		900005: jimeng,
		900010: matchingCustom,
		900011: otherCustom,
		900012: typeSafe,
	}
	t.Cleanup(func() {
		channelsIDM = previous
		channelSyncLock.Unlock()
	})

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			kept, emptiedBy := filterCandidateIDs(testCase.ids, testCase.modelName, testCase.filters)
			if testCase.wantKept == nil {
				assert.Nil(t, kept)
			} else {
				assert.Equal(t, testCase.wantKept, kept)
			}
			assert.Equal(t, testCase.wantEmpty, emptiedBy)
		})
	}
}

func TestChannelSatisfiesFilters(t *testing.T) {
	alphaSetting := `{"task_plugin_key":"alpha"}`
	alpha := &Channel{Id: 1, Type: constant.ChannelTypeTaskPlugin, Setting: &alphaSetting}
	ordinary := &Channel{Id: 2, Type: constant.ChannelTypeOpenAI}
	custom := &Channel{Id: 3, Type: constant.ChannelTypeAdvancedCustom}
	custom.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{{
				IncomingPath: "/v1/chat/completions",
				Models:       []string{"gpt-4"},
			}},
		},
	})
	typeSafe := &Channel{Id: 4, Type: constant.ChannelTypeTypeSafe}

	ok, kind := ChannelSatisfiesFilters(nil, "gpt-4", nil)
	assert.False(t, ok)
	assert.Equal(t, dto.ChannelFilterKind(""), kind)

	ok, kind = ChannelSatisfiesFilters(alpha, "shared", identityFilters("alpha", nil))
	require.True(t, ok)
	assert.Equal(t, dto.ChannelFilterKind(""), kind)

	ok, kind = ChannelSatisfiesFilters(alpha, "shared", identityFilters("beta", nil))
	assert.False(t, ok)
	assert.Equal(t, dto.FilterTaskPluginIdentity, kind)

	ok, kind = ChannelSatisfiesFilters(ordinary, "gpt-4", []dto.ChannelFilter{{
		Kind:        dto.FilterRequestPath,
		RequestPath: "/v1/chat/completions",
	}})
	require.True(t, ok)
	assert.Equal(t, dto.ChannelFilterKind(""), kind)

	ok, kind = ChannelSatisfiesFilters(custom, "gpt-4", []dto.ChannelFilter{{
		Kind:        dto.FilterRequestPath,
		RequestPath: "/v1/responses",
	}})
	assert.False(t, ok)
	assert.Equal(t, dto.FilterRequestPath, kind)

	ok, kind = ChannelSatisfiesFilters(typeSafe, "jev-1.13", []dto.ChannelFilter{{
		Kind:        dto.FilterRequestPath,
		RequestPath: "/v1/systemone",
	}})
	assert.True(t, ok)
	assert.Equal(t, dto.ChannelFilterKind(""), kind)

	ok, kind = ChannelSatisfiesFilters(typeSafe, "jev-1.13", []dto.ChannelFilter{{
		Kind:        dto.FilterRequestPath,
		RequestPath: "/v1/chat/completions",
	}})
	assert.False(t, ok)
	assert.Equal(t, dto.FilterRequestPath, kind)
}

func TestClaudeCodeClientFilter(t *testing.T) {
	restrictedSetting := kitdto.ChannelOtherSettings{ClaudeCodeOnly: true}
	restricted := &Channel{Id: 900100, Type: constant.ChannelTypeAnthropic, Status: common.ChannelStatusEnabled}
	restricted.SetOtherSettings(restrictedSetting)
	open := &Channel{Id: 900101, Type: constant.ChannelTypeAnthropic, Status: common.ChannelStatusEnabled}

	claudeFilter := func(isClaudeCode bool) []dto.ChannelFilter {
		return []dto.ChannelFilter{{Kind: dto.FilterClaudeCodeClient, ClaudeCodeClient: isClaudeCode}}
	}

	tests := []struct {
		name       string
		channel    *Channel
		isClaude   bool
		want       bool
		wantKind   dto.ChannelFilterKind
		annotation string
	}{
		{
			name:     "restricted channel accepts a Claude Code client",
			channel:  restricted,
			isClaude: true,
			want:     true,
		},
		{
			name:     "restricted channel refuses any other client",
			channel:  restricted,
			isClaude: false,
			want:     false,
			wantKind: dto.FilterClaudeCodeClient,
		},
		{
			name:     "channel without the switch accepts any client",
			channel:  open,
			isClaude: false,
			want:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, kind := ChannelSatisfiesFilters(tc.channel, "claude-sonnet-4-5", claudeFilter(tc.isClaude))
			assert.Equal(t, tc.want, ok)
			assert.Equal(t, tc.wantKind, kind)
		})
	}

	// The restriction is not limited to Messages: any relay path is refused.
	t.Run("restriction applies to every relay path", func(t *testing.T) {
		for _, path := range []string{"/v1/messages", "/v1/chat/completions", "/v1/responses", "/v1/embeddings"} {
			filters := append(claudeFilter(false), dto.ChannelFilter{Kind: dto.FilterRequestPath, RequestPath: path})
			ok, kind := ChannelSatisfiesFilters(restricted, "claude-sonnet-4-5", filters)
			assert.False(t, ok, path)
			assert.Equal(t, dto.FilterClaudeCodeClient, kind, path)
		}
	})

	t.Run("cache filtering keeps only allowed candidates", func(t *testing.T) {
		oldChannels := channelsIDM
		t.Cleanup(func() { channelsIDM = oldChannels })
		channelsIDM = map[int]*Channel{900100: restricted, 900101: open}

		kept, emptiedBy := filterCandidateIDs([]int{900100, 900101}, "claude-sonnet-4-5", claudeFilter(false))
		assert.Equal(t, []int{900101}, kept)
		assert.Equal(t, dto.ChannelFilterKind(""), emptiedBy)

		kept, emptiedBy = filterCandidateIDs([]int{900100}, "claude-sonnet-4-5", claudeFilter(true))
		assert.Equal(t, []int{900100}, kept)
		assert.Equal(t, dto.ChannelFilterKind(""), emptiedBy)

		kept, emptiedBy = filterCandidateIDs([]int{900100}, "claude-sonnet-4-5", claudeFilter(false))
		assert.Empty(t, kept)
		assert.Equal(t, dto.FilterClaudeCodeClient, emptiedBy)
	})
}
