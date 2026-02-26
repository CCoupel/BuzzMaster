import '@testing-library/jest-dom'

// jsdom does not implement scrollIntoView — polyfill it globally
window.HTMLElement.prototype.scrollIntoView = function () {}
