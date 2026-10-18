import { Link } from '@tanstack/react-router'
import { ApiError } from '../api/queries'
import { formatCountry } from '../lib/format'

export function ListSkeleton() {
  return (
    <div role="status" aria-label="Loading jobs" className="animate-pulse">
      <div className="mx-4 my-3 h-4 w-20 rounded bg-gray-200" />
      {Array.from({ length: 6 }, (_, i) => (
        <div key={i} className="space-y-2 border-b border-gray-100 px-4 py-3">
          <div className="h-4 w-2/3 rounded bg-gray-200" />
          <div className="h-3 w-1/2 rounded bg-gray-100" />
        </div>
      ))}
    </div>
  )
}

export function DetailSkeleton() {
  return (
    <div role="status" aria-label="Loading job" className="animate-pulse space-y-3 p-6">
      <div className="h-7 w-1/2 rounded bg-gray-200" />
      <div className="h-4 w-1/3 rounded bg-gray-100" />
      <div className="h-4 w-1/4 rounded bg-gray-100" />
      <div className="h-32 rounded bg-gray-100" />
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
    <div className="space-y-3 p-6 text-center">
      <p className="text-gray-700">{message}</p>
      <button type="button" onClick={onClear} className="text-blue-700 underline">
        Clear filters
      </button>
    </div>
  )
}

export function ErrorState({ error, onRetry }: { error: Error; onRetry: () => void }) {
  const problem = error instanceof ApiError ? error.problem : undefined
  return (
    <div role="alert" className="space-y-2 p-6 text-center">
      <p className="font-medium">{problem?.title ?? 'Something went wrong'}</p>
      <p className="text-gray-700">{problem?.detail ?? 'The request failed. Please try again.'}</p>
      <button
        type="button"
        onClick={onRetry}
        className="rounded border border-gray-300 px-3 py-1 hover:bg-gray-50"
      >
        Retry
      </button>
    </div>
  )
}

export function JobGone() {
  return (
    <div className="space-y-3 p-6">
      <p className="text-gray-700">This job is no longer available.</p>
      <Link
        from="/"
        to="/"
        search={(prev) => ({ ...prev, job: undefined })}
        className="text-blue-700"
      >
        ‹ Back to results
      </Link>
    </div>
  )
}
