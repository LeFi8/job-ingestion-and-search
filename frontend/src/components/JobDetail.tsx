import { Link } from '@tanstack/react-router'
import type { JobDetail as Job } from '../api/client'
import { formatLocation, formatPostedOn, formatUsdAnnual } from '../lib/format'

const companyTypeLabels: Record<Job['company_type'], string | undefined> = {
  direct_employer: 'Direct employer',
  consulting_agency: 'Consulting agency',
  unknown: undefined,
}

export function JobDetail({ job }: { job: Job | undefined }) {
  if (!job) return null
  const companyType = companyTypeLabels[job.company_type]

  return (
    <article className="space-y-3 p-6">
      <Link
        from="/"
        to="/"
        search={(prev) => ({ ...prev, job: undefined })}
        className="text-sm text-blue-700 lg:hidden"
      >
        ‹ Back to results
      </Link>
      <h2 className="text-2xl font-semibold">{job.title}</h2>
      <p className="text-gray-700">
        {[job.company, formatLocation([job.city, job.region, job.country], job.remote), companyType]
          .filter(Boolean)
          .join(' · ')}
      </p>
      <p className="flex flex-wrap items-center gap-2">
        <span>{job.salary_display}</span>
        <span className="text-gray-600">≈ {formatUsdAnnual(job.salary_usd_annual)}</span>
        {job.currency_inferred && (
          <span className="rounded bg-amber-100 px-2 py-0.5 text-xs text-amber-900">
            currency assumed
          </span>
        )}
      </p>
      <p className="text-sm text-gray-600">
        {job.posted_on ? 'Posted ' : ''}
        {formatPostedOn(job.posted_on)}
      </p>
      <hr className="border-gray-200" />
      <p className="whitespace-pre-line">{job.description}</p>
    </article>
  )
}
