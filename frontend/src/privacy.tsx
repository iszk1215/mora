import React from 'react'

import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '@/components/ui/table'

const cookieRows = [
  {
    name: 'morasessionid',
    purpose: 'Session management: keeps you signed in and tracks the active session.',
    duration: 'Browser session',
  },
  {
    name: 'csrf_token',
    purpose: 'CSRF protection: validates that form submissions and API requests are legitimate.',
    duration: 'Browser session',
  },
  {
    name: 'oauth_state',
    purpose: 'OAuth login: validates the login state during the redirect flow.',
    duration: '10 minutes',
  },
]

export const PrivacyPage = (): React.JSX.Element => {
  return (
    <div className="mx-auto max-w-3xl">
      <h1 className="text-3xl font-bold mb-6">Privacy Policy</h1>

      <h2 className="text-xl font-semibold mb-3">Cookie Usage</h2>
      <p className="text-muted-foreground mb-4 leading-relaxed">
        This site uses only necessary (functional) cookies to operate correctly, including
        session management, CSRF protection, and OAuth login state validation. It does not
        use tracking, analytics, or advertising cookies, and it does not use third-party
        cookies. All cookies below are set by this site itself (first-party).
      </p>

      <Table className="mb-4">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Purpose</TableHead>
            <TableHead>Duration</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {cookieRows.map((row) => (
            <TableRow key={row.name}>
              <TableCell className="font-mono">{row.name}</TableCell>
              <TableCell>{row.purpose}</TableCell>
              <TableCell>{row.duration}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <h2 className="text-xl font-semibold mb-3">Controlling Cookies</h2>
      <p className="text-muted-foreground leading-relaxed">
        You can delete or block cookies through your browser settings. Note that these
        cookies are required for the site to function, so removing or blocking them may
        prevent you from signing in or using the site.
      </p>
    </div>
  )
}