import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

export const api = createClient<paths>({ baseUrl: window.location.origin })

export type JobCard = components['schemas']['JobCard']
export type JobDetail = components['schemas']['JobDetail']
export type JobsPage = components['schemas']['JobsPage']
export type Problem = components['schemas']['Problem']
export type Sort = NonNullable<paths['/api/jobs']['get']['parameters']['query']>['sort']
