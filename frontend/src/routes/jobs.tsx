import { getRouteApi } from '@tanstack/react-router'
import { ApiError, perPage, useJob, useJobs } from '../api/queries'
import { JobDetail } from '../components/JobDetail'
import { JobList } from '../components/JobList'
import { SearchBar } from '../components/SearchBar'
import {
  DetailSkeleton,
  EmptyState,
  ErrorState,
  JobGone,
  ListSkeleton,
  PageOutOfRange,
} from '../components/States'
import { pageCount } from './jobsSearch'

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
      <header className="border-b bg-card">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
          <span className="text-lg font-bold tracking-tight text-primary">JobSearch</span>
          <div className="w-full min-w-0 sm:w-auto sm:flex-1">
            <SearchBar facets={jobs.data?.facets.country ?? []} />
          </div>
        </div>
      </header>
      <main className="mx-auto grid min-h-0 w-full max-w-7xl flex-1 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <section
          aria-label="Job list"
          className={`${showDetailOnly ? 'hidden lg:block' : ''} min-h-0 overflow-y-auto p-4`}
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
          ) : jobs.data.items.length === 0 ? (
            <PageOutOfRange page={search.page} lastPage={pageCount(jobs.data.total, perPage)} />
          ) : (
            <JobList page={jobs.data} selectedId={selectedId} isFetching={jobs.isFetching} />
          )}
        </section>
        <section
          aria-label="Job details"
          className={`${showDetailOnly ? '' : 'hidden lg:block'} min-h-0 overflow-y-auto p-4 lg:pl-2`}
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
          ) : job.isLoading || (selectedId === undefined && jobs.isPending) ? (
            <DetailSkeleton />
          ) : (
            <JobDetail job={job.data} />
          )}
        </section>
      </main>
    </div>
  )
}
