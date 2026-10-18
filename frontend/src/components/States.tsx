import { Link } from '@tanstack/react-router'
import { ChevronLeft, SearchX, TriangleAlert } from 'lucide-react'
import { ApiError } from '../api/queries'
import { formatCountry } from '../lib/format'
import { Button } from './ui/button'
import { Skeleton } from './ui/skeleton'

export function ListSkeleton() {
  return (
    <div role="status" aria-label="Loading jobs" className="space-y-2">
      <Skeleton className="mx-1 mb-3 h-4 w-20" />
      {Array.from({ length: 6 }, (_, i) => (
        <div key={i} className="flex gap-3 rounded-xl border bg-card p-3">
          <Skeleton className="size-10 rounded-lg" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-3 w-1/2" />
            <Skeleton className="h-5 w-24 rounded-full" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function DetailSkeleton() {
  return (
    <div role="status" aria-label="Loading job" className="space-y-5 rounded-xl border bg-card p-6">
      <div className="flex gap-4">
        <Skeleton className="size-12 rounded-lg" />
        <div className="flex-1 space-y-2">
          <Skeleton className="h-7 w-1/2" />
          <Skeleton className="h-4 w-1/3" />
        </div>
      </div>
      <div className="flex gap-2">
        <Skeleton className="h-5 w-40 rounded-full" />
        <Skeleton className="h-5 w-28 rounded-full" />
      </div>
      <Skeleton className="h-32" />
    </div>
  )
}

export function EmptyState({
  q,
  country,
  onClear,
}: {
  q: string
  country: string | undefined
  onClear: () => void
}) {
  const message = `No jobs${q ? ` match '${q}'` : ''}${country ? ` in ${formatCountry(country)}` : ''}`
  return (
    <div className="flex flex-col items-center gap-3 rounded-xl border bg-card p-8 text-center">
      <SearchX className="size-8 text-muted-foreground" aria-hidden="true" />
      <p className="font-medium">{message}</p>
      <Button variant="outline" onClick={onClear}>
        Clear filters
      </Button>
    </div>
  )
}

export function ErrorState({ error, onRetry }: { error: Error; onRetry: () => void }) {
  const problem = error instanceof ApiError ? error.problem : undefined
  return (
    <div
      role="alert"
      className="flex flex-col items-center gap-2 rounded-xl border bg-card p-8 text-center"
    >
      <TriangleAlert className="size-8 text-destructive" aria-hidden="true" />
      <p className="font-medium">{problem?.title ?? 'Something went wrong'}</p>
      <p className="text-sm text-muted-foreground">
        {problem?.detail ?? 'The request failed. Please try again.'}
      </p>
      <Button variant="outline" onClick={onRetry} className="mt-1">
        Retry
      </Button>
    </div>
  )
}

export function PageOutOfRange({ page, lastPage }: { page: number; lastPage: number }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-xl border bg-card p-8 text-center">
      <p className="font-medium">{`Page ${String(page)} is past the end of the results.`}</p>
      <Button asChild variant="outline">
        <Link from="/" to="/" search={(prev) => ({ ...prev, page: lastPage, job: undefined })}>
          <ChevronLeft aria-hidden="true" />
          {`Go to page ${String(lastPage)}`}
        </Link>
      </Button>
    </div>
  )
}

export function JobGone() {
  return (
    <div className="flex flex-col items-center gap-3 rounded-xl border bg-card p-8 text-center">
      <p className="font-medium">This job is no longer available.</p>
      <Button asChild variant="outline">
        <Link from="/" to="/" search={(prev) => ({ ...prev, job: undefined })}>
          <ChevronLeft aria-hidden="true" />
          Back to results
        </Link>
      </Button>
    </div>
  )
}
