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
/**
 * Billing an image request per generated image.
 *
 * The expression form is the one the visual expression editor already renders
 * as "price per image": a fixed price multiplied by the validated output image
 * count. The engine settles it against the image count the upstream actually
 * returned, so a request that produced fewer images is charged less.
 *
 * Semantics live in pkg/billingexpr (validateFixedPricingTree accepts
 * `image_count` as a fixed-price multiplier) and in pkg/billingexpr/expr.md.
 */
const PER_IMAGE_LABEL = 'image'

// `tier("image", fixed(0.04)) * image_count`, tolerating the parentheses and
// whitespace the expression editor may keep from an earlier edit.
const PER_IMAGE_EXPRESSION =
  /^\s*\(*\s*tier\(\s*"(?:[^"\\]|\\.)*"\s*,\s*fixed\(\s*(\d+(?:\.\d+)?)\s*\)\s*\)*\s*\*\s*image_count\s*\)*\s*$/

export function buildPerImageExpression(price: string): string {
  return `tier(${JSON.stringify(PER_IMAGE_LABEL)}, fixed(${price})) * image_count`
}

/** The per-image unit price an expression charges, or undefined for any other shape. */
export function readPerImagePrice(expression: string): string | undefined {
  const match = PER_IMAGE_EXPRESSION.exec(expression.trim())
  return match ? match[1] : undefined
}
