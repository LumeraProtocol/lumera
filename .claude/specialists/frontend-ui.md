---
id: frontend-ui
name: Frontend & UI
triggers:
  keywords: [react, vue, component, css, accessibility, state, dom, browser, frontend]
  feature_patterns: ["*ui*", "*frontend*", "*component*", "*page*", "*view*"]
applies_to: [architect, coder]
---

## When to apply

This specialist is relevant when the project involves building user interfaces, web frontends, component libraries, or any browser-rendered application. Look for features involving pages, views, components, forms, or user interaction.

## Guidance for architect

- Define a component hierarchy before building — page → layout → feature → primitive
- Choose state management approach based on complexity: local state for simple, global store for shared
- Establish a consistent styling strategy (CSS modules, utility classes, CSS-in-JS) and stick with it
- Plan the routing structure to match user mental models, not implementation structure
- Design loading and error states for every async operation — they are not afterthoughts
- Define accessibility requirements upfront (WCAG level, screen reader support, keyboard navigation)
- Separate data fetching from presentation components

## Guidance for coder

- Build components from the bottom up — primitives first, composition second
- Keep components focused: if a component does more than one thing, split it
- Handle all three async states: loading, success, error — never leave a blank screen on failure
- Use semantic HTML elements (nav, main, article, button) — not div for everything
- Ensure keyboard navigation works for all interactive elements
- Test user interactions (click, type, submit), not implementation details (state values, method calls)
- Debounce expensive operations triggered by user input (search, resize, scroll)
- Avoid direct DOM manipulation when using a framework — let the framework manage the DOM

## Review checklist

- [ ] Component hierarchy documented
- [ ] Loading and error states for all async operations
- [ ] Keyboard navigation works for interactive elements
- [ ] Semantic HTML used appropriately
- [ ] No accessibility violations (run automated checker)
- [ ] Components are reusable and focused on single responsibilities
- [ ] Forms have validation feedback
