import { expect, test } from 'vitest'
import { formatPostedOn, formatUsdAnnual } from './format'

test('formatPostedOn keeps the calendar date west of UTC', () => {
  expect(Intl.DateTimeFormat().resolvedOptions().timeZone).toBe('America/New_York')
  expect(formatPostedOn('2023-10-03')).toBe('Oct 3, 2023')
})

test('formatPostedOn without a date', () => {
  expect(formatPostedOn(undefined)).toBe('Date not provided')
})

test('formatUsdAnnual rounds to whole dollars', () => {
  expect(formatUsdAnnual(104000)).toBe('$104,000/yr')
})
