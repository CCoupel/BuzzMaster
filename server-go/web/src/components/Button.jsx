import { motion } from 'framer-motion'
import './Button.css'

export default function Button({
  children,
  variant = 'primary',
  size = 'md',
  icon,
  disabled = false,
  loading = false,
  onClick,
  className = '',
  as,
  ...props
}) {
  const Component = as === 'span' ? motion.span : motion.button

  return (
    <Component
      className={`btn btn-${variant} btn-${size} ${className}`}
      disabled={as !== 'span' ? (disabled || loading) : undefined}
      onClick={onClick}
      whileHover={{ scale: disabled ? 1 : 1.02 }}
      whileTap={{ scale: disabled ? 1 : 0.98 }}
      {...props}
    >
      {loading ? (
        <span className="btn-spinner" />
      ) : (
        <>
          {icon && <span className="btn-icon">{icon}</span>}
          {children}
        </>
      )}
    </Component>
  )
}
