import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse, http } from 'msw'
import { expect, test } from 'vitest'
import { createQueryClient } from '../api/queries'
import { createAppRouter } from '../router'
import {
  customerSuccessManager,
  cybersecuritySpecialist,
  engPage,
  jobDetails,
  listPage,
} from '../test/fixtures'
import { requests, server } from '../test/server'

function installApi() {
  server.use(
    http.get('/api/jobs', ({ request }) =>
      HttpResponse.json(new URL(request.url).searchParams.has('q') ? engPage : listPage),
    ),
    http.get('/api/jobs/:id', ({ params }) => {
      const job = jobDetails[String(params.id)]
      if (job) return HttpResponse.json(job)
      return HttpResponse.json(
        { title: 'Not Found', status: 404, detail: 'Job not found.' },
        { status: 404, headers: { 'Content-Type': 'application/problem+json' } },
      )
    }),
  )
}

async function renderApp(url: string) {
  const history = createMemoryHistory({ initialEntries: [url] })
  render(
    <QueryClientProvider client={createQueryClient()}>
      <RouterProvider router={createAppRouter(history)} />
    </QueryClientProvider>,
  )
  await screen.findByRole('region', { name: 'Job list' })
  return { history, urlParams: () => new URLSearchParams(history.location.search) }
}

const jobsRequests = () => requests.filter((u) => u.pathname === '/api/jobs')
const lastJobsRequest = () => jobsRequests().at(-1)?.searchParams
const jobList = () => screen.getByRole('region', { name: 'Job list' })
const jobDetail = () => screen.getByRole('region', { name: 'Job details' })

test('lists jobs, shows the selected job and searches by title', async () => {
  installApi()
  const user = userEvent.setup()
  const { urlParams } = await renderApp('/')

  const projectManagerLink = await within(jobList()).findByRole('link', { name: /Project Manager/ })
  expect(projectManagerLink).toHaveTextContent('$105,600/yr')
  const sort = screen.getByRole('combobox', { name: 'Sort by' })
  expect(within(sort).queryByRole('option', { name: 'Relevance' })).toBeNull()
  expect(await within(jobDetail()).findByText('80000.00 GBP / year')).toBeInTheDocument()
  expect(within(jobDetail()).getByText('currency assumed')).toBeInTheDocument()

  await user.click(within(jobList()).getByRole('link', { name: /Growth Marketing Manager/ }))
  expect(await within(jobDetail()).findByText('Date not provided')).toBeInTheDocument()

  await user.type(screen.getByRole('searchbox', { name: 'Search job titles' }), 'eng')
  await waitFor(() => {
    expect(urlParams().get('q')).toBe('eng')
  })
  expect(urlParams().has('job')).toBe(false)
  await waitFor(() => {
    expect(lastJobsRequest()?.get('q')).toBe('eng')
  })
  expect(jobsRequests().map((u) => u.searchParams.get('q'))).not.toContain('e')
  const marks = await within(jobList()).findAllByText('Engineer', { selector: 'mark' })
  expect(marks.length).toBeGreaterThan(0)
})

test('the URL drives the request and back/forward restores each view', async () => {
  installApi()
  const user = userEvent.setup()
  const { history, urlParams } = await renderApp('/?q=eng&page=2')

  await waitFor(() => {
    const req = lastJobsRequest()
    expect(req?.get('q')).toBe('eng')
    expect(req?.get('page')).toBe('2')
    expect(req?.get('per_page')).toBe('20')
  })

  await screen.findByRole('option', { name: 'Canada (1)' })
  await user.selectOptions(screen.getByRole('combobox', { name: 'Country' }), 'CA')
  await waitFor(() => {
    expect(urlParams().get('country')).toBe('CA')
  })
  expect(urlParams().get('q')).toBe('eng')
  expect(urlParams().has('page')).toBe(false)
  await waitFor(() => {
    expect(lastJobsRequest()?.get('country')).toBe('CA')
  })

  await user.selectOptions(screen.getByRole('combobox', { name: 'Sort by' }), 'salary_desc')
  await waitFor(() => {
    expect(urlParams().get('sort')).toBe('salary_desc')
  })
  expect(urlParams().get('q')).toBe('eng')
  expect(urlParams().get('country')).toBe('CA')

  history.back()
  history.back()
  await waitFor(() => {
    expect(history.location.search).toBe('?q=eng&page=2')
  })
  await waitFor(() => {
    const req = lastJobsRequest()
    expect(req?.get('q')).toBe('eng')
    expect(req?.get('page')).toBe('2')
    expect(req?.has('country')).toBe(false)
    expect(req?.has('sort')).toBe(false)
  })
})

test('selecting a job loads its details by id, even when it is not in the list', async () => {
  installApi()
  const user = userEvent.setup()
  const { urlParams } = await renderApp('/')

  const links = await within(jobList()).findAllByRole('link')
  const second = links[1]
  if (!second) throw new Error('expected at least two job links')
  await user.click(second)

  await waitFor(() => {
    expect(urlParams().get('job')).toBe(customerSuccessManager.id)
  })
  expect(requests.map((u) => u.pathname)).toContain(`/api/jobs/${customerSuccessManager.id}`)
  expect(
    await within(jobDetail()).findByText('Help customers succeed with our platform.'),
  ).toBeInTheDocument()

  cleanup()
  await renderApp(`/?job=${cybersecuritySpecialist.id}`)
  expect(await within(jobDetail()).findByText('Monitor security events.')).toBeInTheDocument()
})

test('special characters reach the API intact', async () => {
  installApi()
  const user = userEvent.setup()
  await renderApp('/')

  await user.type(screen.getByRole('searchbox', { name: 'Search job titles' }), 'C++ & 100%')
  await waitFor(() => {
    expect(lastJobsRequest()?.get('q')).toBe('C++ & 100%')
  })
})
