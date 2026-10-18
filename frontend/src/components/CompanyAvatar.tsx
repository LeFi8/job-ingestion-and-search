import { cn } from '../lib/utils'

const palette = [
  'bg-primary',
  'bg-emerald-600',
  'bg-amber-600',
  'bg-rose-600',
  'bg-cyan-600',
  'bg-slate-600',
]

function colorFor(name: string) {
  let hash = 0
  for (const ch of name) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return palette[hash % palette.length]
}

export function CompanyAvatar({ company, size = 'md' }: { company: string; size?: 'md' | 'lg' }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'grid shrink-0 place-items-center rounded-lg font-semibold text-white',
        size === 'lg' ? 'size-12 text-lg' : 'size-10',
        colorFor(company),
      )}
    >
      {company.charAt(0).toUpperCase()}
    </span>
  )
}
