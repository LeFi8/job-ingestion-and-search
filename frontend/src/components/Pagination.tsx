import { Link } from '@tanstack/react-router'

const maxPages = 200

export function Pagination({
  page,
  total,
  perPage,
}: {
  page: number
  total: number
  perPage: number
}) {
  const pages = Math.min(Math.ceil(total / perPage), maxPages)
  if (pages <= 1) return null

  return (
    <nav aria-label="Pagination" className="flex items-center justify-between px-4 py-3 text-sm">
      {page > 1 ? (
        <Link from="/" to="/" search={(prev) => ({ ...prev, page: page - 1 })}>
          ‹ Prev
        </Link>
      ) : (
        <span className="text-gray-400">‹ Prev</span>
      )}
      <span>{`Page ${String(page)} of ${String(pages)}`}</span>
      {page < pages ? (
        <Link from="/" to="/" search={(prev) => ({ ...prev, page: page + 1 })}>
          Next ›
        </Link>
      ) : (
        <span className="text-gray-400">Next ›</span>
      )}
    </nav>
  )
}
