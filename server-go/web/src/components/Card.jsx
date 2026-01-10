import { motion } from 'framer-motion'
import './Card.css'

export default function Card({
  children,
  variant = 'default',
  padding = 'md',
  hover = false,
  onClick,
  className = '',
  style = {},
  ...props
}) {
  const Component = onClick ? motion.button : motion.div

  return (
    <Component
      className={`card card-${variant} card-padding-${padding} ${hover ? 'card-hover' : ''} ${className}`}
      onClick={onClick}
      style={style}
      whileHover={hover ? { y: -4, scale: 1.01 } : undefined}
      whileTap={onClick ? { scale: 0.98 } : undefined}
      {...props}
    >
      {children}
    </Component>
  )
}

export function CardHeader({ children, className = '', onClick }) {
  return (
    <div
      className={`card-header ${className}`}
      onClick={onClick}
      style={onClick ? { cursor: 'pointer' } : undefined}
    >
      {children}
    </div>
  )
}

export function CardBody({ children, className = '' }) {
  return <div className={`card-body ${className}`}>{children}</div>
}

export function CardFooter({ children, className = '' }) {
  return <div className={`card-footer ${className}`}>{children}</div>
}
