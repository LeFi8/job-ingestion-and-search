import type { JobsPage } from '../api/client'
import { perPage } from '../api/queries'
import { JobListItem } from './JobListItem'
import { Pagination } from './Pagination'

export function JobList({
  page,
  selectedId,
  isFetching,
}: {
  page: JobsPage | undefined
  selectedId: string | undefined
  isFetching: boolean
}) {
  return (
    <div aria-busy={isFetching} className={isFetching ? 'opacity-60 transition-opacity' : ''}>
      <p aria-live="polite" className="px-1 pb-2 text-sm text-muted-foreground">
        {page ? `${String(page.total)} ${page.total === 1 ? 'job' : 'jobs'}` : ''}
      </p>
      <ul className="space-y-2">
        {page?.items.map((job) => (
          <li key={job.id}>
            <JobListItem job={job} selected={job.id === selectedId} />
          </li>
        ))}
      </ul>
      {page && <Pagination page={page.page} total={page.total} perPage={perPage} />}
    </div>
  )
}
