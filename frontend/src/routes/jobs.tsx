import { getRouteApi } from '@tanstack/react-router'
import { ApiError, useJob, useJobs } from '../api/queries'
import { JobDetail } from '../components/JobDetail'
import { JobList } from '../components/JobList'
import { SearchBar } from '../components/SearchBar'
import { DetailSkeleton, EmptyState, ErrorState, JobGone, ListSkeleton } from '../components/States'

const route = getRouteApi('/')

export function JobsPage() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const jobs = useJobs(search)
  const selectedId = search.job ?? jobs.data?.items[0]?.id
  const job = useJob(selectedId)
  const showDetailOnly = search.job !== undefined

  return (
    <div className="flex h-dvh flex-col">
      <header className="border-b border-gray-200 p-4">
        <SearchBar facets={jobs.data?.facets.country ?? []} />
      </header>
      <main className="grid min-h-0 flex-1 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <section
          aria-label="Job list"
          className={`${showDetailOnly ? 'hidden lg:block' : ''} overflow-y-auto border-r border-gray-200`}
        >
          {jobs.isError ? (
            <ErrorState
              error={jobs.error}
              onRetry={() => {
                void jobs.refetch()
              }}
            />
          ) : jobs.isPending ? (
            <ListSkeleton />
          ) : jobs.data.total === 0 ? (
            <EmptyState
              q={search.q.trim()}
              country={search.country}
              onClear={() => {
                void navigate({ search: {} })
              }}
            />
          ) : (
            <JobList page={jobs.data} selectedId={selectedId} isFetching={jobs.isFetching} />
          )}
        </section>
        <section
          aria-label="Job details"
          className={`${showDetailOnly ? '' : 'hidden lg:block'} overflow-y-auto`}
        >
          {job.error instanceof ApiError && job.error.status === 404 ? (
            <JobGone />
          ) : job.isError ? (
            <ErrorState
              error={job.error}
              onRetry={() => {
                void job.refetch()
              }}
            />
          ) : job.isLoading ? (
            <DetailSkeleton />
          ) : (
            <JobDetail job={job.data} />
          )}
        </section>
      </main>
    </div>
  )
}
