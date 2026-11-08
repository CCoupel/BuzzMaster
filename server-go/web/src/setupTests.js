import '@testing-library/jest-dom'

// jsdom does not implement scrollIntoView — polyfill it globally
if (typeof window !== 'undefined') {
  window.HTMLElement.prototype.scrollIntoView = function () {}
}
