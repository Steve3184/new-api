/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapSupportEnabled(t *testing.T) {
	original := common.SupportEnabled
	t.Cleanup(func() {
		common.SupportEnabled = original
	})

	require.NoError(t, updateOptionMap("SupportEnabled", "false"))
	require.False(t, common.SupportEnabled)

	require.NoError(t, updateOptionMap("SupportEnabled", "true"))
	require.True(t, common.SupportEnabled)
}
