import { ReactNode } from 'react'
import { Link } from 'react-router-dom'

interface LinkProps {
    to: string
    children: ReactNode
}

export const DefaultLink = (props: LinkProps): JSX.Element => {
    return <Link className="text-blue-600 dark:text-blue-500 hover:underline" to={props.to}>{props.children}</Link>
}

export const HeaderLink = (props: LinkProps): JSX.Element => {
    return <Link to={props.to}>{props.children}</Link>
}

interface ExternalLinkProps {
    href: string
    children: ReactNode
}

export const ExternalLink = (props: ExternalLinkProps): JSX.Element => {
    return <a className="text-blue-600 dark:text-blue-500 hover:underline" href={props.href}>{props.children}</a>
}
