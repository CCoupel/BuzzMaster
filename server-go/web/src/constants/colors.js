/**
 * Shared color constants for BuzzControl
 */

// QCM answer colors - used for multiple choice questions
export const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

// Mapping from button press (A, B, C, D) to QCM color
export const BUTTON_TO_QCM_COLOR = {
  'A': 'RED',
  'B': 'GREEN',
  'C': 'YELLOW',
  'D': 'BLUE',
}

// Question categories with icons and colors
export const CATEGORIES = {
  GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
  ENTERTAINMENT: { label: 'Divertissement', icon: '🎭', color: '#ec4899' },
  HISTORY: { label: 'Histoire', icon: '📜', color: '#eab308' },
  ARTS: { label: 'Arts & Litterature', icon: '🎨', color: '#a855f7' },
  SCIENCE: { label: 'Sciences & Nature', icon: '🔬', color: '#22c55e' },
  SPORTS: { label: 'Sports & Loisirs', icon: '⚽', color: '#f97316' },
  FOOD: { label: 'Gastronomie', icon: '🍽️', color: '#991b1b' },
  ANIMALS: { label: 'Animaux', icon: '🐾', color: '#78716c' },
}

// Answer colors for unassigned players (TeamsPage)
export const ANSWER_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}
