import { setupServer } from 'msw/node'

export const server = setupServer()

export const requests: URL[] = []

server.events.on('request:start', ({ request }) => {
  const url = new URL(request.url)
  if (url.pathname.startsWith('/api/')) requests.push(url)
})
