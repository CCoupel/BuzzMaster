import { useMemo, useState } from 'react'
import { motion } from 'framer-motion'
import './CategoryBalance.css'

// Import categories from QuestionsPage
import { CATEGORIES } from '../pages/QuestionsPage'

/**
 * CategoryBalance - Displays category distribution balance
 * Shows diverging bars for questions count and points per category
 * Color coding: green = balanced, orange = warning, red = imbalanced
 */
export default function CategoryBalance({ questions = [] }) {
  const [hoveredCategory, setHoveredCategory] = useState(null)

  // Calculate statistics per category
  const stats = useMemo(() => {
    // Group questions by category
    const categoryData = {}

    questions.forEach(q => {
      const cat = q.CATEGORY || 'UNCATEGORIZED'
      if (!categoryData[cat]) {
        categoryData[cat] = { count: 0, points: 0, questions: [] }
      }
      categoryData[cat].count += 1
      categoryData[cat].points += parseInt(q.POINTS) || 0
      categoryData[cat].questions.push(q)
    })

    // Only keep categories that have questions and are in CATEGORIES
    const representedCategories = Object.keys(categoryData).filter(
      cat => cat !== 'UNCATEGORIZED' && CATEGORIES[cat]
    )

    if (representedCategories.length === 0) {
      return null
    }

    // Calculate totals and averages
    const totalQuestions = questions.filter(q => q.CATEGORY && CATEGORIES[q.CATEGORY]).length
    const totalPoints = representedCategories.reduce((sum, cat) => sum + categoryData[cat].points, 0)
    const avgQuestions = totalQuestions / representedCategories.length
    const avgPoints = totalPoints / representedCategories.length

    // Calculate deviations and colors for each category
    const categories = representedCategories.map(cat => {
      const data = categoryData[cat]
      const questionsDev = data.count - avgQuestions
      const questionsDevPercent = avgQuestions > 0 ? (questionsDev / avgQuestions) * 100 : 0
      const pointsDev = data.points - avgPoints
      const pointsDevPercent = avgPoints > 0 ? (pointsDev / avgPoints) * 100 : 0

      // Color based on deviation percentage
      const getColor = (devPercent) => {
        const absPercent = Math.abs(devPercent)
        if (absPercent <= 25) return 'balanced'      // Green
        if (absPercent <= 50) return 'warning'       // Orange
        return 'imbalanced'                          // Red
      }

      return {
        key: cat,
        ...CATEGORIES[cat],
        count: data.count,
        points: data.points,
        questionsDev,
        questionsDevPercent,
        pointsDev,
        pointsDevPercent,
        questionsColor: getColor(questionsDevPercent),
        pointsColor: getColor(pointsDevPercent),
      }
    })

    // Sort by category order (as defined in CATEGORIES)
    const categoryOrder = Object.keys(CATEGORIES)
    categories.sort((a, b) => categoryOrder.indexOf(a.key) - categoryOrder.indexOf(b.key))

    // Find max deviation for scaling bars
    const maxQuestionsDev = Math.max(...categories.map(c => Math.abs(c.questionsDev)), 1)
    const maxPointsDev = Math.max(...categories.map(c => Math.abs(c.pointsDev)), 1)

    return {
      categories,
      totalQuestions,
      totalPoints,
      avgQuestions: avgQuestions.toFixed(1),
      avgPoints: avgPoints.toFixed(0),
      maxQuestionsDev,
      maxPointsDev,
    }
  }, [questions])

  if (!stats || stats.categories.length === 0) {
    return null
  }

  return (
    <div className="category-balance">
      <div className="balance-header">
        <span className="balance-title">Equilibre des categories</span>
        <span className="balance-summary">
          {stats.totalQuestions} questions • {stats.totalPoints} pts •
          Moy: {stats.avgQuestions} quest. / {stats.avgPoints} pts
        </span>
      </div>

      <div className="balance-categories">
        {stats.categories.map((cat, index) => (
          <motion.div
            key={cat.key}
            className="balance-category"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: index * 0.05 }}
            onMouseEnter={() => setHoveredCategory(cat.key)}
            onMouseLeave={() => setHoveredCategory(null)}
          >
            {/* Category header */}
            <div className="category-header">
              <span
                className="category-icon"
                style={{ backgroundColor: cat.color }}
                title={cat.label}
              >
                {cat.icon}
              </span>
              <span className="category-values">
                {cat.count} / {cat.points}
              </span>
            </div>

            {/* Questions bar (diverging from center) */}
            <div className="diverging-bar questions-bar">
              <div className="bar-left">
                {cat.questionsDev < 0 && (
                  <motion.div
                    className={`bar-fill left ${cat.questionsColor}`}
                    initial={{ width: 0 }}
                    animate={{
                      width: `${(Math.abs(cat.questionsDev) / stats.maxQuestionsDev) * 100}%`
                    }}
                    transition={{ duration: 0.5, delay: index * 0.05 }}
                  />
                )}
              </div>
              <div className="bar-center" />
              <div className="bar-right">
                {cat.questionsDev > 0 && (
                  <motion.div
                    className={`bar-fill right ${cat.questionsColor}`}
                    initial={{ width: 0 }}
                    animate={{
                      width: `${(Math.abs(cat.questionsDev) / stats.maxQuestionsDev) * 100}%`
                    }}
                    transition={{ duration: 0.5, delay: index * 0.05 }}
                  />
                )}
              </div>
            </div>

            {/* Points bar (diverging from center) */}
            <div className="diverging-bar points-bar">
              <div className="bar-left">
                {cat.pointsDev < 0 && (
                  <motion.div
                    className={`bar-fill left ${cat.pointsColor}`}
                    initial={{ width: 0 }}
                    animate={{
                      width: `${(Math.abs(cat.pointsDev) / stats.maxPointsDev) * 100}%`
                    }}
                    transition={{ duration: 0.5, delay: index * 0.05 + 0.1 }}
                  />
                )}
              </div>
              <div className="bar-center" />
              <div className="bar-right">
                {cat.pointsDev > 0 && (
                  <motion.div
                    className={`bar-fill right ${cat.pointsColor}`}
                    initial={{ width: 0 }}
                    animate={{
                      width: `${(Math.abs(cat.pointsDev) / stats.maxPointsDev) * 100}%`
                    }}
                    transition={{ duration: 0.5, delay: index * 0.05 + 0.1 }}
                  />
                )}
              </div>
            </div>

            {/* Tooltip on hover */}
            {hoveredCategory === cat.key && (
              <motion.div
                className="balance-tooltip"
                initial={{ opacity: 0, y: 5 }}
                animate={{ opacity: 1, y: 0 }}
              >
                <div className="tooltip-header" style={{ borderColor: cat.color }}>
                  <span className="tooltip-icon">{cat.icon}</span>
                  <span className="tooltip-label">{cat.label}</span>
                </div>
                <div className="tooltip-section">
                  <div className="tooltip-row">
                    <span>Questions:</span>
                    <span>{cat.count} (moy: {stats.avgQuestions})</span>
                  </div>
                  <div className={`tooltip-row deviation ${cat.questionsColor}`}>
                    <span>Ecart:</span>
                    <span>
                      {cat.questionsDev >= 0 ? '+' : ''}{cat.questionsDev.toFixed(1)}
                      ({cat.questionsDevPercent >= 0 ? '+' : ''}{cat.questionsDevPercent.toFixed(0)}%)
                    </span>
                  </div>
                </div>
                <div className="tooltip-section">
                  <div className="tooltip-row">
                    <span>Points:</span>
                    <span>{cat.points} (moy: {stats.avgPoints})</span>
                  </div>
                  <div className={`tooltip-row deviation ${cat.pointsColor}`}>
                    <span>Ecart:</span>
                    <span>
                      {cat.pointsDev >= 0 ? '+' : ''}{cat.pointsDev.toFixed(0)}
                      ({cat.pointsDevPercent >= 0 ? '+' : ''}{cat.pointsDevPercent.toFixed(0)}%)
                    </span>
                  </div>
                </div>
                <div className="tooltip-status">
                  {cat.questionsColor === 'balanced' && cat.pointsColor === 'balanced' ? (
                    <span className="status-ok">✓ Equilibre</span>
                  ) : cat.questionsDev > 0 || cat.pointsDev > 0 ? (
                    <span className="status-over">⚠ Sur-represente</span>
                  ) : (
                    <span className="status-under">⚠ Sous-represente</span>
                  )}
                </div>
              </motion.div>
            )}
          </motion.div>
        ))}
      </div>
    </div>
  )
}
