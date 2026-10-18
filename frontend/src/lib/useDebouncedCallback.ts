import { useCallback, useEffect, useRef } from 'react'

export function useDebouncedCallback<A extends unknown[]>(fn: (...args: A) => void, ms: number) {
  const latestRef = useRef(fn)
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    latestRef.current = fn
  })
  useEffect(
    () => () => {
      clearTimeout(timerRef.current)
    },
    [],
  )

  return useCallback(
    (...args: A) => {
      clearTimeout(timerRef.current)
      timerRef.current = setTimeout(() => {
        latestRef.current(...args)
      }, ms)
    },
    [ms],
  )
}
