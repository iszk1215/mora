import React, { useEffect } from 'react'
import { isRouteErrorResponse, ScrollRestoration, useRouteError } from 'react-router'
import { Header } from './main'
import { DefaultLink } from './util'

interface ErrorInfo {
  status?: number
  title: string
  detail?: string
  unexpected: boolean
}

const statusTitle = (status: number): string => {
  if (status === 400) return 'Bad Request'
  if (status === 401) return 'Unauthorized'
  if (status === 403) return 'Forbidden'
  if (status === 404) return 'Not Found'
  if (status === 409) return 'Conflict'
  if (status >= 500) return 'Internal Server Error'
  return 'Error'
}

export const describeError = (error: unknown): ErrorInfo => {
  if (isRouteErrorResponse(error)) {
    return {
      status: error.status,
      title: error.statusText || statusTitle(error.status),
      unexpected: false,
    }
  }
  if (typeof Response !== 'undefined' && error instanceof Response) {
    return {
      status: error.status,
      title: statusTitle(error.status),
      unexpected: false,
    }
  }
  if (error instanceof Error) {
    return {
      title: 'Something went wrong',
      detail: error.message,
      unexpected: true,
    }
  }
  return { title: 'Something went wrong', unexpected: true }
}

interface ErrorContentProps {
  label?: React.ReactNode
  title: string
  children?: React.ReactNode
}

// Chrome-less page content. Pages rendered inside the root layout (e.g.
// NotFoundPage via the catch-all route) must use this directly; rendering
// their own <Header> would duplicate the one from the root layout.
export const ErrorContent = ({ label, title, children }: ErrorContentProps): React.JSX.Element => {
  return (
    <div className="my-16 flex flex-col items-center text-center">
      {label && <p className="text-sm font-semibold text-muted-foreground">{label}</p>}
      <h1 className="text-3xl font-bold">{title}</h1>
      <div className="mt-3 text-muted-foreground">{children}</div>
      <div className="mt-8">
        <DefaultLink to="/">Back to top page</DefaultLink>
      </div>
    </div>
  )
}

export const ErrorPage = (): React.JSX.Element => {
  const error = useRouteError()
  const info = describeError(error)
  useEffect(() => {
    if (info.unexpected) {
      console.error('Unhandled route error:', error)
    }
  }, [error, info.unexpected])
  return (
    <div>
      <ScrollRestoration />
      <Header />
      <div className="w-full sm:w-8/12 m-auto px-4 sm:px-0">
        <ErrorContent label={info.status ?? 'Error'} title={info.title}>
          <p>Sorry, something went wrong.</p>
          {info.detail && info.unexpected && <p className="text-sm">{info.detail}</p>}
        </ErrorContent>
      </div>
    </div>
  )
}

export const NotFoundPage = (): React.JSX.Element => {
  return (
    <ErrorContent label={404} title="Page not found">
      <p>The page you are looking for does not exist or may have been moved.</p>
    </ErrorContent>
  )
}
