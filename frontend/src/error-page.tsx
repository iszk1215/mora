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

interface ErrorLayoutProps {
  heading: React.ReactNode
  title: string
  children?: React.ReactNode
}

const ErrorLayout = ({ heading, title, children }: ErrorLayoutProps): React.JSX.Element => {
  return (
    <div>
      <ScrollRestoration />
      <Header />
      <div className="w-full sm:w-8/12 m-auto px-4 sm:px-0">
        <div className="my-16 flex flex-col items-center text-center">
          <p className="text-6xl font-bold text-destructive">{heading}</p>
          <h1 className="mt-4 text-2xl font-semibold">{title}</h1>
          <div className="mt-2 text-muted-foreground">{children}</div>
          <div className="mt-8">
            <DefaultLink to="/">Back to top page</DefaultLink>
          </div>
        </div>
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
    <ErrorLayout heading={info.status ?? 'Error'} title={info.title}>
      <p>Sorry, something went wrong.</p>
      {info.detail && info.unexpected && <p className="text-sm">{info.detail}</p>}
    </ErrorLayout>
  )
}

export const NotFoundPage = (): React.JSX.Element => {
  return (
    <ErrorLayout heading={404} title="Page not found">
      <p>The page you are looking for does not exist or may have been moved.</p>
    </ErrorLayout>
  )
}
