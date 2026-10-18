import { z } from 'zod'

export const sortValues = ['relevance', 'salary_desc', 'salary_asc', 'date_desc', 'date_asc'] as const

export const jobsSearchSchema = z.object({
  q: z.string().max(200).catch('').default(''),
  country: z
    .string()
    .regex(/^[A-Z]{2}$/)
    .optional()
    .catch(undefined),
  sort: z.enum(sortValues).optional().catch(undefined),
  page: z.coerce.number().int().min(1).max(200).catch(1).default(1),
  job: z.uuid().optional().catch(undefined),
})

export type JobsSearch = z.infer<typeof jobsSearchSchema>

export function parseSearch(s: string): Record<string, string> {
  return Object.fromEntries(new URLSearchParams(s))
}

export function stringifySearch(o: Record<string, unknown>): string {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(o)) {
    if (value === undefined || value === '' || (key === 'page' && value === 1)) continue
    params.set(key, typeof value === 'string' ? value : JSON.stringify(value))
  }
  const s = params.toString()
  return s ? `?${s}` : ''
}
