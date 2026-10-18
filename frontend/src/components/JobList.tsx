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
    <div aria-busy={isFetching}>
      <p aria-live="polite" className="px-4 py-2 text-sm text-gray-600">
        {page ? `${String(page.total)} ${page.total === 1 ? 'job' : 'jobs'}` : ''}
      </p>
      <ul>
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
