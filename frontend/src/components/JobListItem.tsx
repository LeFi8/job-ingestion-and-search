import { Link } from '@tanstack/react-router'
import type { JobCard } from '../api/client'
import { formatLocation, formatUsdAnnual } from '../lib/format'

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
      className={`block border-l-4 px-4 py-3 hover:bg-gray-50 ${
        selected ? 'border-blue-600 bg-blue-50' : 'border-transparent'
      }`}
    >
      <span className="block font-medium">
        {job.title_highlight
          ? withOffsets(job.title_highlight).map(({ text, match, offset }) =>
              match ? <mark key={offset}>{text}</mark> : text,
            )
          : job.title}
      </span>
      <span className="block text-sm text-gray-600">
        {job.company} · {formatLocation([job.city, job.country], job.remote)}
      </span>
      <span className="block text-sm">{formatUsdAnnual(job.salary_usd_annual)}</span>
    </Link>
  )
}
