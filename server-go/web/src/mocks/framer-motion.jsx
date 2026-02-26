// Automatic mock for framer-motion used by vitest in jsdom environment.
// Replaces motion.* components with plain HTML elements, stripping animation props.

const ANIM_PROPS = new Set([
  'animate', 'initial', 'exit', 'variants', 'transition', 'layout', 'layoutId',
  'whileHover', 'whileTap', 'whileFocus', 'whileInView', 'whileDrag',
  'drag', 'dragConstraints', 'onDrag', 'onDragStart', 'onDragEnd',
])

function makeMotionComponent(tag) {
  function MotionComponent({ children, ...props }) {
    const domProps = {}
    for (const key of Object.keys(props)) {
      if (!ANIM_PROPS.has(key)) domProps[key] = props[key]
    }
    const Tag = tag
    return <Tag {...domProps}>{children}</Tag>
  }
  MotionComponent.displayName = `motion.${tag}`
  return MotionComponent
}

export const motion = new Proxy({}, {
  get: (_, tag) => makeMotionComponent(tag),
})

export function AnimatePresence({ children }) {
  return children
}
