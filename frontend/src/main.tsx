import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createBrowserHistory } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createQueryClient } from './api/queries'
import './index.css'
import { createAppRouter } from './router'

const root = document.getElementById('root')
if (!root) throw new Error('missing #root element')

const queryClient = createQueryClient()
const router = createAppRouter(createBrowserHistory())

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
