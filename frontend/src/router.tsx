import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import type { RouterHistory } from '@tanstack/react-router'
import { JobsPage } from './routes/jobs'
import { jobsSearchSchema, parseSearch, stringifySearch } from './routes/jobsSearch'

const rootRoute = createRootRoute()

const jobsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  validateSearch: jobsSearchSchema,
  component: JobsPage,
})

const routeTree = rootRoute.addChildren([jobsRoute])

export function createAppRouter(history: RouterHistory) {
  return createRouter({ routeTree, history, parseSearch, stringifySearch })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
