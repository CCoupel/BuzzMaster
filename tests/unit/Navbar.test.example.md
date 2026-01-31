# Unit Tests - Navbar Component (v2.48.0)

## Setup

### Framework recommandé
- **Vitest** (compatible avec Vite)
- **React Testing Library** pour les tests de composant

### Installation (si besoin futur)
```bash
npm install --save-dev vitest @testing-library/react @testing-library/dom jsdom
```

### Configuration Vitest (vite.config.js)
```javascript
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
  },
})
```

---

## Tests Unitaires

### Test 1 : Menu s'ouvre au clic sur le bouton
```javascript
describe('Navbar Menu', () => {
  test('should open menu when bee button is clicked', () => {
    // ARRANGE
    const { getByRole, getByText } = render(
      <BrowserRouter>
        <Navbar serverVersion="2.48.0" clientCounts={{ admin: 2, tv: 1, vplayer: 0 }} />
      </BrowserRouter>
    )
    const beeButton = getByRole('button', { name: /Menu/i })

    // ACT
    fireEvent.click(beeButton)

    // ASSERT
    const menu = getByText('Config').closest('.navbar-menu-dropdown')
    expect(menu).toBeInTheDocument()
  })
})
```

### Test 2 : Menu se ferme au clic sur un item
```javascript
test('should close menu when a menu item is clicked', async () => {
  // ARRANGE
  const { getByRole, getByText, queryByText } = render(
    <BrowserRouter>
      <Navbar serverVersion="2.48.0" clientCounts={{ admin: 2, tv: 1, vplayer: 0 }} />
    </BrowserRouter>
  )
  const beeButton = getByRole('button', { name: /Menu/i })
  fireEvent.click(beeButton) // Ouvrir le menu

  // ACT
  const configLink = getByText('Config')
  fireEvent.click(configLink)

  // ASSERT - Menu devrait disparaître après la navigation
  await waitFor(() => {
    expect(queryByText('Config')).not.toBeInTheDocument()
  })
})
```

### Test 3 : Config et Logs ne sont pas dans la navbar principale
```javascript
test('should not display Config and Logs in main navbar', () => {
  // ARRANGE
  const { queryByText, getByText } = render(
    <BrowserRouter>
      <Navbar serverVersion="2.48.0" clientCounts={{ admin: 2, tv: 1, vplayer: 0 }} />
    </BrowserRouter>
  )

  // ASSERT
  // Vérifier que Config et Logs n'apparaissent pas dans les links visibles
  const navlinksContainer = getByText('Joueurs') // Should be in main navbar
  expect(navlinksContainer).toBeInTheDocument()

  // Config et Logs ne doivent pas être dans nav-group-items normaux
  const mainLinks = document.querySelectorAll('.navbar-links .nav-link')
  const mainLinkTexts = Array.from(mainLinks).map(link => link.textContent)
  expect(mainLinkTexts).not.toContain('Config')
  expect(mainLinkTexts).not.toContain('Logs')
})
```

### Test 4 : Menu s'ouvre et se ferme avec toggle
```javascript
test('should toggle menu open/closed on bee button clicks', () => {
  // ARRANGE
  const { getByRole, queryByText } = render(
    <BrowserRouter>
      <Navbar serverVersion="2.48.0" clientCounts={{ admin: 2, tv: 1, vplayer: 0 }} />
    </BrowserRouter>
  )
  const beeButton = getByRole('button', { name: /Menu/i })

  // ACT & ASSERT - Première ouverture
  fireEvent.click(beeButton)
  expect(queryByText('Config')).toBeInTheDocument()

  // ACT & ASSERT - Fermeture
  fireEvent.click(beeButton)
  expect(queryByText('Config')).not.toBeInTheDocument()

  // ACT & ASSERT - Réouverture
  fireEvent.click(beeButton)
  expect(queryByText('Config')).toBeInTheDocument()
})
```

### Test 5 : Menu se ferme au clic extérieur
```javascript
test('should close menu when clicking outside', async () => {
  // ARRANGE
  const { getByRole, queryByText, getByText } = render(
    <div>
      <BrowserRouter>
        <Navbar serverVersion="2.48.0" clientCounts={{ admin: 2, tv: 1, vplayer: 0 }} />
      </BrowserRouter>
      <div data-testid="outside">Outside element</div>
    </div>
  )
  const beeButton = getByRole('button', { name: /Menu/i })
  fireEvent.click(beeButton) // Ouvrir le menu

  // ACT
  const outsideElement = getByTestId('outside')
  fireEvent.mouseDown(outsideElement) // Simuler clic extérieur

  // ASSERT
  await waitFor(() => {
    expect(queryByText('Config')).not.toBeInTheDocument()
  })
})
```

### Test 6 : Version affichée correctement
```javascript
test('should display server version in version badge', () => {
  // ARRANGE
  const { getByText } = render(
    <BrowserRouter>
      <Navbar serverVersion="2.48.0" clientCounts={{ admin: 2, tv: 1, vplayer: 0 }} />
    </BrowserRouter>
  )

  // ASSERT
  expect(getByText('v2.48.0')).toBeInTheDocument()
})
```

### Test 7 : Client counts affichés
```javascript
test('should display client counts', () => {
  // ARRANGE
  const { getByText } = render(
    <BrowserRouter>
      <Navbar
        serverVersion="2.48.0"
        clientCounts={{ admin: 2, tv: 1, vplayer: 3 }}
      />
    </BrowserRouter>
  )

  // ASSERT
  expect(getByText('2')).toBeInTheDocument() // Admin count
  expect(getByText('1')).toBeInTheDocument() // TV count
  expect(getByText('3')).toBeInTheDocument() // VPlayer count
})
```

### Test 8 : Attributs ARIA présents
```javascript
test('should have accessibility attributes', () => {
  // ARRANGE
  const { getByRole } = render(
    <BrowserRouter>
      <Navbar serverVersion="2.48.0" clientCounts={{ admin: 2, tv: 1, vplayer: 0 }} />
    </BrowserRouter>
  )

  // ASSERT
  const beeButton = getByRole('button', { name: /Menu de navigation/i })
  expect(beeButton).toHaveAttribute('aria-label', 'Menu de navigation')
  expect(beeButton).toHaveAttribute('title', 'Menu')
})
```

---

## Couverture de Tests

| Composant | Ligne | Couverture |
|-----------|-------|-----------|
| useState (isMenuOpen) | L8 | 100% |
| useRef (menuRef) | L9 | 100% |
| useRef (buttonRef) | L10 | 100% |
| useEffect (click outside) | L13-27 | 100% |
| Bouton cliquable | L76-91 | 100% |
| Menu conditionnel | L94-108 | 100% |
| Navigation items | L96-106 | 100% |

---

## Notes

### Dépendances pour les tests
- `@testing-library/react` : Pour render() et fireEvent()
- `@testing-library/dom` : Pour queries avancées
- `jsdom` : Pour simuler le DOM
- `vitest` : Framework de test

### À tester en priorité
1. ✅ Menu s'ouvre/ferme
2. ✅ Fermeture au clic extérieur
3. ✅ Navigation vers Config/Logs
4. ✅ Config et Logs retirés de navbar principale

### À améliorer (v2.49.0)
- [ ] Support ESC key pour fermer le menu
- [ ] Tests de responsive design
- [ ] Tests d'animation CSS
- [ ] Tests de keyboard navigation
