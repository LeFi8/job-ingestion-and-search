import { getRouteApi } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useState } from 'react'
import type { JobsPage } from '../api/client'
import { formatCountry } from '../lib/format'
import { useDebouncedCallback } from '../lib/useDebouncedCallback'
import { sortValues } from '../routes/jobsSearch'
import { Input } from './ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select'

const route = getRouteApi('/')
const allCountries = 'all'

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
  const countries =
    search.country && !facets.some((f) => f.value === search.country)
      ? [...facets, { value: search.country, count: 0 }]
      : facets
  const sortOptions = hasQuery ? sortValues : sortValues.filter((s) => s !== 'relevance')

  return (
    <div className="flex flex-wrap gap-2">
      <label className="relative basis-full sm:min-w-48 sm:flex-1 sm:basis-auto">
        <span className="sr-only">Search job titles</span>
        <Search
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          type="search"
          value={draft}
          placeholder="Search job titles…"
          onChange={(e) => {
            changeQuery(e.target.value)
          }}
          onKeyDown={(e) => {
            if (e.key === 'Escape') changeQuery('')
          }}
          className="h-9 bg-muted pl-9"
        />
      </label>
      <Select
        value={search.country ?? allCountries}
        onValueChange={(value) => {
          void navigate({
            search: (prev) => ({
              ...prev,
              country: value === allCountries ? undefined : value,
              page: undefined,
              job: undefined,
            }),
          })
        }}
      >
        <SelectTrigger
          aria-label="Country"
          className="flex-1 bg-muted data-[size=default]:h-9 sm:min-w-40 sm:flex-none"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={allCountries}>All countries</SelectItem>
          {countries.map((f) => (
            <SelectItem key={f.value} value={f.value}>
              {`${formatCountry(f.value)} (${String(f.count)})`}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={search.sort ?? (hasQuery ? 'relevance' : 'date_desc')}
        onValueChange={(value) => {
          const sort = sortValues.find((s) => s === value)
          void navigate({
            search: (prev) => ({ ...prev, sort, page: undefined, job: undefined }),
          })
        }}
      >
        <SelectTrigger
          aria-label="Sort by"
          className="flex-1 bg-muted data-[size=default]:h-9 sm:min-w-36 sm:flex-none"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {sortOptions.map((s) => (
            <SelectItem key={s} value={s}>
              {sortLabels[s]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
