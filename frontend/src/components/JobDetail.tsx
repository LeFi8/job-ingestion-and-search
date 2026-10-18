import { Link } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'
import type { JobDetail as Job } from '../api/client'
import { formatLocation, formatPostedOn, formatUsdAnnual } from '../lib/format'
import { CompanyAvatar } from './CompanyAvatar'
import { Badge } from './ui/badge'

const companyTypeLabels: Record<Job['company_type'], string | undefined> = {
  direct_employer: 'Direct employer',
  consulting_agency: 'Consulting agency',
  unknown: undefined,
}

export function JobDetail({ job }: { job: Job | undefined }) {
  if (!job) return null
  const companyType = companyTypeLabels[job.company_type]

  return (
    <article className="space-y-5 rounded-xl border bg-card p-6">
      <Link
        from="/"
        to="/"
        search={(prev) => ({ ...prev, job: undefined })}
        className="inline-flex items-center gap-1 text-sm font-medium text-primary lg:hidden"
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
        Back to results
      </Link>
      <header className="flex gap-4">
        <CompanyAvatar company={job.company} size="lg" />
        <div className="min-w-0 space-y-1">
          <h2 className="text-2xl font-semibold tracking-tight">{job.title}</h2>
          <p className="text-muted-foreground">
            {[job.company, formatLocation([job.city, job.region, job.country]), companyType]
              .filter(Boolean)
              .join(' · ')}
          </p>
        </div>
      </header>
      <div className="flex flex-wrap items-center gap-2">
        <Badge className="bg-emerald-50 text-emerald-800">
          <span>{job.salary_display}</span>
          <span>· ≈ {formatUsdAnnual(job.salary_usd_annual)}</span>
        </Badge>
        {job.currency_inferred && (
          <Badge className="bg-amber-100 text-amber-900">currency assumed</Badge>
        )}
        {job.remote && <Badge className="bg-sky-50 text-sky-800">Remote</Badge>}
        <Badge variant="secondary">
          {job.posted_on ? 'Posted ' : ''}
          {formatPostedOn(job.posted_on)}
        </Badge>
      </div>
      <hr />
      <p className="leading-relaxed whitespace-pre-line">{job.description}</p>
    </article>
  )
}
