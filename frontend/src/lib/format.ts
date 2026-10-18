const usd = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  maximumFractionDigits: 0,
})

const postedOn = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
  timeZone: 'UTC',
})

export function formatUsdAnnual(n: number): string {
  return `${usd.format(n)}/yr`
}

export function formatLocation(parts: (string | undefined)[], remote: boolean): string {
  const place = parts.filter(Boolean).join(', ')
  if (!remote) return place
  return place ? `${place} · Remote` : 'Remote'
}

export function formatPostedOn(d?: string): string {
  if (d === undefined) return 'Date not provided'
  return postedOn.format(new Date(d))
}
