import { Link } from '@tanstack/react-router'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { pageCount } from '../routes/jobs.search'
import { Button } from './ui/button'

export function Pagination({
  page,
  total,
  perPage,
}: {
  page: number
  total: number
  perPage: number
}) {
  const pages = pageCount(total, perPage)
  if (pages <= 1) return null

  return (
    <nav aria-label="Pagination" className="flex items-center justify-between py-3 text-sm">
      {page > 1 ? (
        <Button asChild variant="outline" size="sm">
          <Link from="/" to="/" search={(prev) => ({ ...prev, page: page - 1, job: undefined })}>
            <ChevronLeft aria-hidden="true" />
            Prev
          </Link>
        </Button>
      ) : (
        <Button variant="outline" size="sm" disabled>
          <ChevronLeft aria-hidden="true" />
          Prev
        </Button>
      )}
      <span className="text-muted-foreground">{`Page ${String(page)} of ${String(pages)}`}</span>
      {page < pages ? (
        <Button asChild variant="outline" size="sm">
          <Link from="/" to="/" search={(prev) => ({ ...prev, page: page + 1, job: undefined })}>
            Next
            <ChevronRight aria-hidden="true" />
          </Link>
        </Button>
      ) : (
        <Button variant="outline" size="sm" disabled>
          Next
          <ChevronRight aria-hidden="true" />
        </Button>
      )}
    </nav>
  )
}
