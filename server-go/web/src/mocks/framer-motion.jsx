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

// Cache components per tag — the Proxy `get` trap fires on every property
// access (`motion.div`, `motion.button`, ...), including on every single
// render of any component that reads it (e.g. `const Component = onClick ?
// motion.button : motion.div` in Card.jsx/Button.jsx). Without this cache,
// each access minted a BRAND NEW function component, so React saw a
// different `type` on every render and fully unmounted + remounted the
// element (and its children) instead of patching it in place — silently
// discarding any DOM node a test had captured across a state update (stale
// `.isConnected === false` references, `fireEvent.click` becoming a no-op).
// Found while investigating #136 (ConfigPage.ai.test.jsx failures unrelated
// to the render-loop hang) — reproduced identically before AND after that
// fix, confirming this bug lives here, not in ConfigPage.jsx.
const motionComponentCache = new Map()

export const motion = new Proxy({}, {
  get: (_, tag) => {
    if (!motionComponentCache.has(tag)) {
      motionComponentCache.set(tag, makeMotionComponent(tag))
    }
    return motionComponentCache.get(tag)
  },
})

export function AnimatePresence({ children }) {
  return children
}
