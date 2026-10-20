import { describe, expect, test } from 'vitest'
import { jobsSearchSchema, parseSearch, stringifySearch } from './jobs.search'

const parse = (search: string) => jobsSearchSchema.parse(parseSearch(search))

const jobId = '0e1cba53-25c9-4a39-b39b-4a4755ef158d'

describe('jobsSearchSchema', () => {
  test.each([
    ['', { q: '', page: 1 }],
    ['?q=123', { q: '123', page: 1 }],
    ['?q=C%2B%2B%20%26%20100%25', { q: 'C++ & 100%', page: 1 }],
    ['?country=us&sort=bogus&page=-3&job=nope', { q: '', page: 1 }],
    ['?page=abc', { q: '', page: 1 }],
    ['?page=201', { q: '', page: 1 }],
    [`?q=${'a'.repeat(201)}`, { q: '', page: 1 }],
    ['?sort=relevance', { q: '', page: 1 }],
    ['?q=%20&sort=relevance', { q: ' ', page: 1 }],
    ['?q=eng&sort=relevance', { q: 'eng', sort: 'relevance', page: 1 }],
    [
      `?q=eng&country=CA&sort=salary_desc&page=3&job=${jobId}`,
      { q: 'eng', country: 'CA', sort: 'salary_desc', page: 3, job: jobId },
    ],
  ])('parses %j', (input, want) => {
    expect(parse(input)).toEqual(want)
  })
})

describe('stringifySearch', () => {
  test('encodes special characters and round-trips through parseSearch', () => {
    const search = stringifySearch({ q: 'C++ & 100%', page: 1, country: undefined })

    expect(search).toMatch(/^\?q=C%2B%2B(\+|%20)%26(\+|%20)100%25$/)
    expect(parseSearch(search).q).toBe('C++ & 100%')
  })

  test('returns an empty string for defaults', () => {
    expect(stringifySearch({ q: '', page: 1, sort: undefined })).toBe('')
  })
})
