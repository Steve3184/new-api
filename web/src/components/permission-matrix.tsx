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
import { useTranslation } from 'react-i18next'

import { Checkbox } from '@/components/ui/checkbox'
import type {
  AdminPermissionMatrix,
  PermissionResourceDef,
} from '@/lib/admin-permissions'

type PermissionMatrixProps = {
  resources: PermissionResourceDef[]
  value: AdminPermissionMatrix
  onChange: (value: AdminPermissionMatrix) => void
  disabled?: boolean
}

// PermissionMatrix renders one checkbox per resource action of a permission
// catalog, with the catalog's translated labels and descriptions.
export function PermissionMatrix(props: PermissionMatrixProps) {
  const { t } = useTranslation()
  return (
    <div className='space-y-3'>
      {props.resources.map((resource) => (
        <div
          key={resource.resource}
          className='space-y-2 rounded-md border p-3'
        >
          <div className='text-sm font-medium'>{t(resource.label_key)}</div>
          <div className='space-y-2'>
            {resource.actions.map((option) => (
              <label key={option.action} className='flex items-start gap-3'>
                <Checkbox
                  checked={
                    props.value[resource.resource]?.[option.action] === true
                  }
                  disabled={props.disabled}
                  onCheckedChange={(checked) => {
                    props.onChange({
                      ...props.value,
                      [resource.resource]: {
                        ...props.value[resource.resource],
                        [option.action]: checked === true,
                      },
                    })
                  }}
                />
                <span className='flex flex-col gap-1'>
                  <span className='text-sm font-medium'>
                    {t(option.label_key)}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {t(option.description_key)}
                  </span>
                </span>
              </label>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
