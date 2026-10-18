import { Link } from '@tanstack/react-router'
import type { JobCard } from '../api/client'
import { formatLocation, formatUsdAnnual } from '../lib/format'
import { CompanyAvatar } from './CompanyAvatar'
import { cn } from '../lib/utils'
import { Badge } from './ui/badge'

type Segment = NonNullable<JobCard['title_highlight']>[number]

function withOffsets(segments: Segment[]) {
  let offset = 0
  return segments.map((segment) => {
    const positioned = { ...segment, offset }
    offset += segment.text.length
    return positioned
  })
}

export function JobListItem({ job, selected }: { job: JobCard; selected: boolean }) {
  return (
    <Link
      from="/"
      to="/"
      search={(prev) => ({ ...prev, job: job.id })}
      aria-current={selected ? 'true' : undefined}
      className={cn(
        'flex gap-3 rounded-xl border bg-card p-3 transition-colors hover:border-primary/40',
        selected && 'border-primary bg-primary-subtle ring-1 ring-primary',
      )}
    >
      <CompanyAvatar company={job.company} />
      <span className="min-w-0 space-y-1">
        <span className="block font-semibold text-primary">
          {job.title_highlight
            ? withOffsets(job.title_highlight).map(({ text, match, offset }) =>
                match ? (
                  <mark key={offset} className="rounded-sm bg-amber-100 text-inherit">
                    {text}
                  </mark>
                ) : (
                  text
                ),
              )
            : job.title}
        </span>
        <span className="block text-sm text-muted-foreground">
          {[job.company, formatLocation([job.city, job.country])].filter(Boolean).join(' · ')}
        </span>
        <span className="flex flex-wrap gap-1.5 pt-1">
          <Badge className="bg-emerald-50 text-emerald-800">
            {formatUsdAnnual(job.salary_usd_annual)}
          </Badge>
          {job.remote && <Badge className="bg-sky-50 text-sky-800">Remote</Badge>}
        </span>
      </span>
    </Link>
  )
}
