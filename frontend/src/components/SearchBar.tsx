import { getRouteApi } from '@tanstack/react-router'
import { useState } from 'react'
import type { JobsPage } from '../api/client'
import { useDebouncedCallback } from '../lib/useDebouncedCallback'
import { sortValues } from '../routes/jobsSearch'

const route = getRouteApi('/')
const regionNames = new Intl.DisplayNames(['en'], { type: 'region' })

const sortLabels: Record<(typeof sortValues)[number], string> = {
  relevance: 'Relevance',
  date_desc: 'Newest',
  date_asc: 'Oldest',
  salary_desc: 'Highest salary',
  salary_asc: 'Lowest salary',
}

export function SearchBar({ facets }: { facets: JobsPage['facets']['country'] }) {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [draft, setDraft] = useState(search.q)
  const [syncedQ, setSyncedQ] = useState(search.q)
  if (search.q !== syncedQ) {
    setSyncedQ(search.q)
    setDraft(search.q)
  }

  const commitQuery = useDebouncedCallback((q: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        q,
        sort: !q.trim() && search.sort === 'relevance' ? undefined : search.sort,
        page: undefined,
        job: undefined,
      }),
      replace: true,
    })
  }, 250)

  const changeQuery = (q: string) => {
    setDraft(q)
    commitQuery(q)
  }

  const hasQuery = search.q.trim() !== ''
  const sortOptions = hasQuery ? sortValues : sortValues.filter((s) => s !== 'relevance')

  return (
    <div className="flex flex-wrap gap-2">
      <label className="min-w-48 flex-1">
        <span className="sr-only">Search job titles</span>
        <input
          type="search"
          value={draft}
          placeholder="Search job titles…"
          onChange={(e) => {
            changeQuery(e.target.value)
          }}
          onKeyDown={(e) => {
            if (e.key === 'Escape') changeQuery('')
          }}
          className="w-full rounded border border-gray-300 px-3 py-2"
        />
      </label>
      <label>
        <span className="sr-only">Country</span>
        <select
          value={search.country ?? ''}
          onChange={(e) => {
            void navigate({
              search: (prev) => ({
                ...prev,
                country: e.target.value || undefined,
                page: undefined,
                job: undefined,
              }),
            })
          }}
          className="rounded border border-gray-300 px-3 py-2"
        >
          <option value="">All countries</option>
          {facets.map((f) => (
            <option key={f.value} value={f.value}>
              {`${regionNames.of(f.value) ?? f.value} (${String(f.count)})`}
            </option>
          ))}
        </select>
      </label>
      <label>
        <span className="sr-only">Sort by</span>
        <select
          value={search.sort ?? (hasQuery ? 'relevance' : 'date_desc')}
          onChange={(e) => {
            const sort = sortValues.find((s) => s === e.target.value)
            void navigate({
              search: (prev) => ({ ...prev, sort, page: undefined, job: undefined }),
            })
          }}
          className="rounded border border-gray-300 px-3 py-2"
        >
          {sortOptions.map((s) => (
            <option key={s} value={s}>
              {sortLabels[s]}
            </option>
          ))}
        </select>
      </label>
    </div>
  )
}
