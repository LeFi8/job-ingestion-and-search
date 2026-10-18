import { QueryClient, keepPreviousData, useQuery } from '@tanstack/react-query'
import type { JobsSearch } from '../routes/jobsSearch'
import { api } from './client'
import type { Problem } from './client'

export const perPage = 20

export class ApiError extends Error {
  readonly status: number
  readonly problem: Problem | undefined

  constructor(status: number, problem?: Problem) {
    super(problem?.detail ?? problem?.title ?? `Request failed with status ${String(status)}`)
    this.name = 'ApiError'
    this.status = status
    this.problem = problem
  }
}

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: (failureCount, error) =>
          error instanceof ApiError && error.status >= 500 && failureCount < 2,
      },
    },
  })
}

export function useJobs(s: JobsSearch) {
  const q = s.q.trim()
  return useQuery({
    queryKey: ['jobs', q, s.country, s.sort, s.page],
    queryFn: async ({ signal }) => {
      const { data, error, response } = await api.GET('/api/jobs', {
        params: {
          query: {
            page: s.page,
            per_page: perPage,
            ...(q ? { q } : {}),
            ...(s.country ? { country: [s.country] } : {}),
            ...(s.sort ? { sort: s.sort } : {}),
          },
        },
        signal,
      })
      if (data === undefined) throw new ApiError(response.status, error)
      return data
    },
    placeholderData: keepPreviousData,
  })
}

export function useJob(id: string | undefined) {
  return useQuery({
    queryKey: ['job', id],
    queryFn: async ({ signal }) => {
      if (id === undefined) throw new Error('useJob: no job id')
      const { data, error, response } = await api.GET('/api/jobs/{id}', {
        params: { path: { id } },
        signal,
      })
      if (data === undefined) throw new ApiError(response.status, error)
      return data
    },
    enabled: id !== undefined,
  })
}
